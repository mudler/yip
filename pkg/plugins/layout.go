package plugins

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"unsafe"

	"github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/partition"
	"github.com/diskfs/go-diskfs/partition/gpt"
	"github.com/gofrs/uuid"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs/v4"
	"golang.org/x/sys/unix"
)

const (
	extMagicOffset1          = 1080
	extMagicOffset2          = 1081
	extMagic1                = 0x53
	extMagic2                = 0xEF
	ext4ExtentFeatureOffset  = 1124
	ext4ExtentFeatureBit     = 0x40
	ext3JournalFeatureOffset = 1084
	ext3JournalFeatureBit    = 0x4
	fat16MagicOffset1        = 54
	fat16MagicOffset2        = 57
	fat32MagicOffset1        = 82
	fat32MagicOffset2        = 90
	fat16Magic               = "FAT"
	fat32Magic               = "FAT32"
	btrfsMagicOffset1        = 0x40
	btrfsMagicOffset2        = 0x48
	btrfsMagic               = "_BHRfS_M"
	xfsMagicOffset1          = 0
	xfsMagicOffset2          = 4
	xfsMagic                 = "XFSB"
	swapMagicSignature       = "SWAPSPACE2"
	OneMiBInBytes            = 1024 * 1024
	Ext4                     = "ext4"
	Ext3                     = "ext3"
	Ext2                     = "ext2"
	Fat                      = "fat"
	Vfat                     = "vfat"
	Fat32                    = "fat32"
	Fat16                    = "fat16"
	Xfs                      = "xfs"
	Btrfs                    = "btrfs"
	Swap                     = "swap"
)

type Disk struct {
	Device  string
	SectorS uint64
	LastS   uint64
	Parts   []Partition
}

type Partition struct {
	Start      uint64
	End        uint64
	Size       uint64
	PLabel     string
	FileSystem string
	FSLabel    string
	PartNumber int
}

type MkfsCall struct {
	part       Partition
	customOpts []string
	dev        string
}

// blkpg_ioctl_arg mirrors struct blkpg_ioctl_arg in <linux/blkpg.h>
type blkpg_ioctl_arg struct {
	Op      int32
	Flags   int32
	Datalen int32
	Data    uintptr // void*
}

// linux/uapi/linux/blkpg.h
type blkpg_partition struct {
	Start   int64 // start sector (512-byte sectors / logical sectors as kernel expects)
	Length  int64 // length in sectors
	Pno     int32 // partition number (1..)
	_       int32 // padding
	Devname [64]byte
	Volname [64]byte
}

// ItsImageFileError is returned when the given path is an image file, not a block device.
// Its done to identify image files when resolving parent disks from partitions.
// So we dont runs the rawPath method twice on image files and get a broken path.
type ItsImageFileError struct{}

func (e ItsImageFileError) Error() string {
	return "the given path is an image file, not a block device"
}

// FilesystemDetector allows mocking filesystem detection in tests.
type FilesystemDetector interface {
	DetectFileSystemType(part *gpt.Partition, d *disk.Disk) (string, error)
}

// RealFilesystemDetector implements FilesystemDetector using real detection logic.
type RealFilesystemDetector struct{}

func (RealFilesystemDetector) DetectFileSystemType(part *gpt.Partition, d *disk.Disk) (string, error) {
	sectorSize := d.LogicalBlocksize
	startOffset := int64(part.Start * uint64(sectorSize))
	// Read first 4KiB from the partition
	buf := make([]byte, 4096)
	n, err := d.Backend.ReadAt(buf, startOffset)
	if err != nil && err != io.EOF {
		return "", err
	}
	buf = buf[:n]

	// ext2/3/4: magic at offset 1080
	if len(buf) > 1125 && buf[extMagicOffset1] == extMagic1 && buf[extMagicOffset2] == extMagic2 {
		// Check for ext4: extents feature (bit 0x40) in feature_incompat at 1124
		if buf[ext4ExtentFeatureOffset]&ext4ExtentFeatureBit != 0 {
			return Ext4, nil
		}
		// Check for ext3: has_journal feature (bit 0x4) in feature_compat at 1084
		if buf[ext3JournalFeatureOffset]&ext3JournalFeatureBit != 0 {
			return Ext3, nil
		}
		// Otherwise, assume ext2
		return Ext2, nil
	}

	// FAT16: "FAT" at offset 54 (FAT12/16)
	if len(buf) > fat16MagicOffset2 && bytes.Equal(buf[fat16MagicOffset1:fat16MagicOffset2], []byte(fat16Magic)) {
		return Fat, nil
	}
	// FAT32: "FAT32   " at offset 82 (FAT32, 8 bytes with spaces)
	// Be more lax with FAT32 detection due to variations in the magic string or extra characters
	if len(buf) > fat32MagicOffset2 && bytes.Contains(buf[fat32MagicOffset1:fat32MagicOffset2], []byte(fat32Magic)) {
		return Fat, nil
	}

	// btrfs: "_BHRfS_M" at offset 0x40
	if len(buf) > 0x47 && bytes.Equal(buf[btrfsMagicOffset1:btrfsMagicOffset2], []byte(btrfsMagic)) {
		return Btrfs, nil
	}

	// xfs: "XFSB" at offset 0
	if len(buf) > 4 && bytes.Equal(buf[xfsMagicOffset1:xfsMagicOffset2], []byte(xfsMagic)) {
		return Xfs, nil
	}

	// swap: "SWAPSPACE2" at end of partition
	swapSig := []byte(swapMagicSignature)
	endOffset := int64((part.End+1)*uint64(sectorSize)) - int64(len(swapSig))
	swapBuf := make([]byte, len(swapSig))
	_, err = d.Backend.ReadAt(swapBuf, endOffset)
	if err == nil && bytes.Equal(swapBuf, swapSig) {
		return Swap, nil
	}
	return "", errors.New("unknown filesystem")
}

var DefaultFilesystemDetector FilesystemDetector = RealFilesystemDetector{}

func Layout(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	l.Info("Running layout plugin")
	if s.Layout.Device == nil {
		l.Debug("Device field empty, skipping layout plugin")
		return nil
	}

	if s.Layout.Device.InitDisk && s.Layout.Device.Path == "" {
		return fmt.Errorf("in order to initialize a disk, a valid device path must be provided")
	}
	if s.Layout.Device.InitDisk && s.Layout.Device.Label != "" {
		return fmt.Errorf("cannot initialize a disk when both path and label are provided, please provide only the device path")
	}
	if s.Layout.Device.InitDisk {
		if _, ok := fs.Stat(s.Layout.Device.Path); ok != nil {
			return fmt.Errorf("cannot initialize disk, path %s does not exist", s.Layout.Device.Path)
		}
		l.Debugf("Initializing disk with path %s", s.Layout.Device.Path)
		d, err := diskfs.Open(s.Layout.Device.Path)
		if err != nil {
			l.Debugf("Disk initialization failed: %s", err)
			return err
		}
		// Do not defer the disk close, we want to close it before returning from this block as other things will open the disk as well.

		var diskName string
		if s.Layout.Device.DiskName != "" {
			diskName = s.Layout.Device.DiskName
		} else {
			diskName = "YIP_DISK"
		}
		// Generate a deterministic GUID based on the disk name
		diskGUID := uuid.NewV5(uuid.NamespaceURL, diskName).String()

		table := &gpt.Table{
			ProtectiveMBR:      true,
			GUID:               diskGUID,
			LogicalSectorSize:  int(d.LogicalBlocksize),
			PhysicalSectorSize: int(d.PhysicalBlocksize),
		}
		err = d.Partition(table)
		if err != nil {
			l.Debugf("Disk initialization failed during partitioning: %s", err)
			_ = d.Close()
			return err
		}
		_ = d.ReReadPartitionTable()

		l.Debugf("Initialized disk with path %s", s.Layout.Device.Path)
		syscall.Sync()
		err = d.Close()
		if err != nil {
			l.Debugf("Disk close failed after initialization: %s", err)
			return err
		}
	}

	var dev Disk
	var err error

	// Validate xfs labels
	for _, part := range s.Layout.Parts {
		if part.FileSystem == "xfs" && len(part.FSLabel) > 12 {
			return fmt.Errorf("xfs filesystem label %s cannot be longer than 12 chars", part.FSLabel)
		}
	}

	l.Debug("Checking layout device information")
	if len(strings.TrimSpace(s.Layout.Device.Path)) > 0 {
		l.Debugf("Using path %s for layout expansion", s.Layout.Device.Path)
		dev, err = FindDiskFromPath(s.Layout.Device.Path, fs)
		if err != nil {
			l.Warnf("Exiting, disk with path %s not found: %s", s.Layout.Device.Path, err.Error())
			return err
		}
	} else if len(strings.TrimSpace(s.Layout.Device.Label)) > 0 {
		l.Debugf("Using label %s for layout expansion", s.Layout.Device.Label)
		dev, err = FindDiskFromLabel(s.Layout.Device.Label, fs)
		if err != nil {
			l.Warnf("Exiting, disk with label %s not found: %s", s.Layout.Device.Label, err.Error())
			return err
		}
	} else {
		l.Warnf("Exiting, no valid device path provided for layout")
		return nil
	}

	l.Debugf("Checking for free space on device %s", dev.Device)
	if !dev.CheckDiskFreeSpaceMiB(32) {
		l.Warnf("Not enough unpartitioned space in disk to operate")
		return nil
	}

	l.Debugf("Checking if more than a partition is marked as bootable on device %s", dev.Device)
	bootableCount := 0
	for _, part := range s.Layout.Parts {
		if part.Bootable {
			bootableCount++
		}
	}
	if bootableCount > 1 {
		l.Warnf("More than one partition is marked as bootable, only one bootable partition is allowed")
	}

	l.Debugf("Going over the partition layout to create partitions on device %s", dev.Device)
	err = dev.AddPartitions(fs, s.Layout.Parts, l, console)
	if err != nil {
		return err
	}

	l.Debugf("Checking for layout expansion on device %s", dev.Device)
	if s.Layout.Expand != nil {
		if s.Layout.Expand.Size == 0 {
			l.Debug("Extending last partition to max space")
		} else {
			l.Debugf("Extending last partition to %d MiB", s.Layout.Expand.Size)
		}
		err := dev.ExpandLastPartition(fs, s.Layout.Expand.Size, console)
		if err != nil {
			l.Error(err.Error())
			return err
		}
		l.Debugf("Extended last partition")
	}
	l.Debugf("All done with layout plugin for device %s", dev.Device)
	return nil
}

func (dev *Disk) AddPartitions(fs vfs.FS, parts []schema.Partition, l logger.Interface, console Console) error {
	if len(parts) == 0 {
		l.Debug("No partitions to add, skipping")
		return nil
	}
	// Open disk
	d, err := diskfs.Open(dev.Device, diskfs.WithOpenMode(diskfs.ReadWrite))
	if err != nil {
		return err
	}
	// We cant defer the close here as we need to close it after writing the partition table so the disk is not in use when formatting partitions

	_ = d.ReReadPartitionTable()

	// Reload the dev.parts with a fresh read
	dev.Parts = GetParts(d)

	// Now get the partition table, once time
	table, err := d.GetPartitionTable()
	if err != nil {
		_ = d.Close()
		return fmt.Errorf("could not get partition table: %w. Maybe the disk is not initialized or doesnt not contain a GPT table", err)
	}
	// Recover here as this will panic if the partition table is not GPT
	gptTable, err := safeTypeAssertion(table)

	if err != nil {
		_ = d.Close()
		return errors.New("only GPT partition tables are supported")
	}

	partitionsToFormat := make([]Partition, 0)

	// Now go over the parts
	for index, p := range parts {
		// For each partition, check if it exists by the partition labeland skip it if so
		if p.PLabel != "" {
			l.Debugf("Checking if partition with PLabel: %s exists on device %s", p.PLabel, dev.Device)
			if dev.MatchPartitionPLabel(p.PLabel) {
				l.Warnf("Partition with PLabel: %s already exists, ignoring", p.PLabel)
				continue
			}
		}

		// Calculate the start, end and size in sectors
		var start uint64
		var end uint64
		var size uint64
		if len(dev.Parts) == 0 {
			// first partition, align to 1Mb
			start = OneMiBInBytes / uint64(dev.SectorS)
		} else {
			// get latest partition end, sum 1
			start = dev.Parts[len(dev.Parts)-1].End + 1
		}

		// part.Size 0 means take over whats left on the disk
		if p.Size == 0 {
			// Remember to add the 1Mb alignment to total size
			// This will be on bytes already no need to transform it
			var sizeUsed = uint64(1024 * 1024)
			for _, partSum := range dev.Parts {
				sizeUsed = sizeUsed + partSum.Size
			}
			// leave 1Mb at the end for backup GPT header
			size = uint64(d.Size) - sizeUsed - uint64(1024*1024)
		} else {
			// Change it to bytes
			// If its the last partition to do, leave 1 Mb at the end for backup GPT header
			if index == len(parts)-1 {
				size = (p.Size * 1024 * 1024) - uint64(1024*1024)
			} else {
				size = p.Size * 1024 * 1024
			}

		}

		end = (size / dev.SectorS) + start - 1

		// Check if there is enough space
		sizeS := MiBToSectors(p.Size, dev.SectorS)
		if start+sizeS > dev.LastS {
			availableMiB := ((dev.LastS - start) * dev.SectorS) / OneMiBInBytes
			_ = d.Close()
			return fmt.Errorf("not enough free space in disk: required %d MiB, available %d MiB", p.Size, availableMiB)
		}

		// default to ext2 if no filesystem provided
		if p.FileSystem == "" {
			p.FileSystem = "ext2"
		}

		var fsType gpt.Type
		var attributes uint64
		switch p.FileSystem {
		case Ext2, Ext3, Ext4, Xfs, Btrfs:
			fsType = gpt.LinuxFilesystem
			// If we identify a COS_GRUB label or bios partition, set it to BIOS boot
			if p.Bootable {
				l.Debugf("Setting bootable attribute for partition %d", len(gptTable.Partitions)+1)
				fsType = gpt.BIOSBoot
				attributes = 0x4 // Set the legacy BIOS bootable attribute
			}
		case Fat16, Fat32, Vfat, Fat:
			fsType = gpt.MicrosoftBasicData
			// If we identify an efi partition, set the required attribute
			if p.Bootable {
				l.Debugf("Setting bootable attribute for partition %d", len(gptTable.Partitions)+1)
				fsType = gpt.EFISystemPartition
				attributes = 0x1 // Set the EFI system partition attribute
			}
		case Swap:
			fsType = gpt.LinuxSwap
		default:
			_ = d.Close()
			return fmt.Errorf("unsupported filesystem type: %s", p.FileSystem)
		}

		part := &gpt.Partition{
			Start:      start,
			End:        end,
			Name:       p.PLabel,
			Type:       fsType,
			Attributes: attributes,
		}
		gptTable.Partitions = append(gptTable.Partitions, part)
		// Now add it to the partitions to format list
		addPart := Partition{
			Start:      start,
			End:        end,
			Size:       size,
			FileSystem: p.FileSystem,
			PLabel:     p.PLabel,
			FSLabel:    p.FSLabel,
			PartNumber: len(gptTable.Partitions), // 1-indexed
		}
		partitionsToFormat = append(partitionsToFormat, addPart)
		// Update dev.Parts to reflect the new partition so we can continue calculating the proper sizes
		dev.Parts = append(dev.Parts, addPart)
		if p.FSLabel != "" {
			l.Debugf("Added partition (fslabel: %s) of size %d MiB on device %s", p.FSLabel, size/(1024*1024), dev.Device)
		} else if p.PLabel != "" {
			l.Debugf("Added partition (label: %s) of size %d MiB on device %s", p.PLabel, size/(1024*1024), dev.Device)
		} else {
			l.Debugf("Added partition %d of size %d MiB on device %s", len(gptTable.Partitions), size/(1024*1024), dev.Device)
		}

	}

	// Now write the partition table back
	err = writePartitionTable(d, gptTable)
	if err != nil {
		_ = d.Close()
		l.Errorf("Error writing partition table: %s", err)
		return err
	}

	// Now try to issue a BLKPG_ADD_PARTITION ioctl to inform the kernel of the new partitions
	// Again, this is best effort, as if the partition is in use, it will fail
	// It should not fail here as we just created the partitions and we are just informing the kernel about it
	// Get the disk file descriptor
	var fd uintptr
	devInfo, err := d.Backend.Stat()
	if err != nil {
		return err
	}

	// Only do this if the backend is a device
	if devInfo.Mode()&os.ModeDevice != 0 {
		osFile, err := d.Backend.Sys()
		if err != nil {
			return err
		}
		fd = osFile.Fd()
		for _, part := range partitionsToFormat {
			var blkpgPart blkpg_partition
			blkpgPart.Start = int64(part.Start)
			blkpgPart.Length = int64((part.End - part.Start) + 1) // inclusive end)
			blkpgPart.Pno = int32(part.PartNumber)

			arg := blkpg_ioctl_arg{
				Op:      int32(unix.BLKPG_ADD_PARTITION),
				Flags:   0,
				Datalen: int32(unsafe.Sizeof(blkpgPart)),
				Data:    uintptr(unsafe.Pointer(&blkpgPart)),
			}
			// Issue ioctl(fd, BLKPG, &arg)
			_, _, err := unix.Syscall(
				unix.SYS_IOCTL,
				fd,
				uintptr(unix.BLKPG),
				uintptr(unsafe.Pointer(&arg)),
			)
			if errors.Is(err, unix.EBUSY) {
				l.Warnf("The partition table was successfully updated on disk, but the kernel could not activate the new partition because the disk is currently in use." +
					"\nThe new partition will become available after the system is rebooted. No data has been lost.")
			} else if err != 0 {
				l.Errorf("Error informing kernel about new partition: %s", err)
				return err
			}
		}
		// Finally, fsync the disk to ensure all changes are written
		if err := unix.Fsync(int(fd)); err != nil {
			return fmt.Errorf("fsync %s failed: %w", dev.Device, err)
		}
	}

	// Close the disk to flush changes
	err = d.Close()
	if err != nil {
		return err
	}

	syscall.Sync()
	_, _ = console.Run("udevadm trigger && udevadm settle")
	// Now format the partitions
	for _, part := range partitionsToFormat {
		l.Debugf("Formatting partition %s on device %s", part.FSLabel, dev.Device)
		out, err := formatPartition(part, dev.Device, console)
		if err != nil {
			l.Errorf("Error formatting partition %s: %s", part.FSLabel, out)
			return err
		}
	}

	return nil
}

func safeTypeAssertion(partitionTable partition.Table) (gptTable *gpt.Table, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("the table of the disk does not seem to be a GPT table")
		}
	}()
	gptTable, ok := partitionTable.(*gpt.Table)
	if !ok {
		return nil, fmt.Errorf("the table of the disk does not seem to be a GPT table")
	}
	return
}

func FindDiskFromPath(path string, fs vfs.FS) (Disk, error) {
	rawPath, err := fs.RawPath(path)
	if err != nil {
		return Disk{}, fmt.Errorf("could not resolve raw path: %w", err)
	}
	d, err := diskfs.Open(rawPath, diskfs.WithOpenMode(diskfs.ReadOnly))
	if err != nil {
		return Disk{}, fmt.Errorf("could not open disk: %w", err)
	}
	// close the disk when done
	defer func() {
		_ = d.Close()
	}()

	// Use d.LogicalBlocksize and d.Size directly
	return Disk{
		Device:  rawPath,
		SectorS: uint64(d.LogicalBlocksize),
		LastS:   uint64(d.Size / d.LogicalBlocksize),
		Parts:   GetParts(d),
	}, nil
}

func FindDiskFromLabel(label string, fs vfs.FS) (Disk, error) {
	var path string
	var err error

	path, err = fs.RawPath(filepath.Join("/dev/disk/by-label", label))
	if err != nil {
		return Disk{}, fmt.Errorf("could not resolve raw path for label %q: %w", label, err)
	}
	// Resolve label to actual full disk path as we can have a partition given to us instead of the disk
	// so the label can be pointing to /dev/sda1 instead of /dev/sda and we want to have sda instead as we want to manage the whole disk
	partDev, err := filepath.EvalSymlinks(path)
	if err != nil {
		return Disk{}, fmt.Errorf("resolve label %q: %w", label, err)
	}

	// Map partition -> parent disk via sysfs directory structure
	diskDev, err := parentDiskFromBlockDev(partDev)

	if err != nil && !errors.Is(err, ItsImageFileError{}) {
		return Disk{}, err
	}
	if errors.Is(err, ItsImageFileError{}) {
		// Its an image file, use partDev as is
		path = diskDev
	} else {
		path, err = fs.RawPath(diskDev)
		if err != nil {
			return Disk{}, fmt.Errorf("could not resolve raw path: %w", err)
		}
	}

	// Read only as we only need the info
	d, err := diskfs.Open(path, diskfs.WithOpenMode(diskfs.ReadOnly))
	if err != nil {
		return Disk{}, fmt.Errorf("could not open disk: %w", err)
	}
	// close the disk when done
	defer func() {
		_ = d.Close()
	}()
	// Use d.LogicalBlocksize and d.Size directly
	return Disk{
		Device:  path,
		SectorS: uint64(d.LogicalBlocksize),
		LastS:   uint64(d.Size / d.LogicalBlocksize),
		Parts:   GetParts(d),
	}, nil
}

func parentDiskFromBlockDev(devPath string) (string, error) {
	// First check if its some kind of image file instead of a block device
	// naively check if the path contains /dev/
	if !strings.HasPrefix(devPath, "/dev/") {
		// Assume its an image file, return as is
		return devPath, ItsImageFileError{}
	}

	// Get the base name of the device
	name := filepath.Base(devPath) // "sda3", "nvme0n1p3", "mmcblk0p2", ...
	fmt.Printf("Deriving parent disk for block device %s\n", devPath)
	sysClass := filepath.Join("/sys/class/block", name)
	fmt.Printf("Deriving parent disk for block device %s\n", sysClass)

	// Read where it points to only if its a symlink
	realSys, err := filepath.EvalSymlinks(sysClass)
	if err != nil {
		return "", fmt.Errorf("sysfs for %s: %w", devPath, err)
	}

	// For partitions, realSys ends with ".../block/<disk>/<partition>"
	// So parent directory basename is "<disk>"
	parentDir := filepath.Dir(realSys)
	parentName := filepath.Base(parentDir)

	// Sanity check: parent should exist in /sys/class/block
	if _, err := os.Stat(filepath.Join("/sys/class/block", parentName)); err != nil {
		return "", fmt.Errorf("derive parent disk for %s (got %s): %w", devPath, parentName, err)
	}

	return filepath.Join("/dev", parentName), nil
}

func (dev *Disk) CheckDiskFreeSpaceMiB(minSpace uint64) bool {
	freeS := dev.computeFreeSpace()
	minSec := MiBToSectors(minSpace, dev.SectorS)
	return freeS >= minSec
}

func (dev *Disk) computeFreeSpace() uint64 {
	if len(dev.Parts) > 0 {
		lastPart := dev.Parts[len(dev.Parts)-1]
		return dev.LastS - (lastPart.Start + lastPart.Size - 1)
	}
	return dev.LastS - (OneMiBInBytes/dev.SectorS - 1)
}

// formatPartition formats the given partition using mkfs commands.
// It expects the disk to be already partitioned.
// It expects the disk to not be open by any other process.
func formatPartition(part Partition, basedevice string, console Console) (string, error) {
	var device string
	// NVMe devices have a different partition naming scheme
	if strings.Contains(basedevice, "nvme") {
		device = fmt.Sprintf("%sp%d", basedevice, part.PartNumber)
	} else {
		device = fmt.Sprintf("%s%d", basedevice, part.PartNumber)
	}
	// We could be also getting here a /dev/disk/by-whatever path, in that case, dont touch it, pass it directly
	if strings.Contains(basedevice, "/dev/disk/") && strings.Contains(basedevice, "/by-") {
		device = basedevice
	}

	mkfs := MkfsCall{part: part, customOpts: []string{}, dev: device}
	return mkfs.Apply(console)
}

func (dev *Disk) ExpandLastPartition(fs vfs.FS, size uint64, console Console) error {
	if len(dev.Parts) == 0 {
		return errors.New("no partition to expand")
	}
	// Open disk and close it when we finish
	d, err := diskfs.Open(dev.Device, diskfs.WithOpenMode(diskfs.ReadWrite))
	if err != nil {
		return err
	}
	// Close it manually at the end before doing the filesystem resize

	// Now try to re-read partition table so kernel sees new partitions
	// This is on a best effort basis, as if the partition is in use, it will fail
	_ = d.ReReadPartitionTable()

	table, err := d.GetPartitionTable()
	if err != nil {
		_ = d.Close()
		return err
	}
	gptTable, ok := table.(*gpt.Table)
	if !ok {
		_ = d.Close()
		return errors.New("only GPT partition tables are supported")
	}
	lastIdx := len(gptTable.Partitions) - 1
	if lastIdx < 0 {
		_ = d.Close()
		return errors.New("no partition to expand")
	}
	part := gptTable.Partitions[lastIdx]
	if part == nil {
		_ = d.Close()
		return errors.New("last partition is nil")
	}
	// Check if the partition is swap as we cannot expand swap partitions
	if part.Type == gpt.LinuxSwap {
		_ = d.Close()
		return errors.New("swap resizing is not supported")
	}

	// Check if partition has fat as we cannot expand fat partitions
	if part.Type == gpt.MicrosoftBasicData || part.Type == gpt.EFISystemPartition {
		_ = d.Close()
		return errors.New("FAT partition resizing is not supported")
	}

	// Check if requested size is less than actual size
	// size in Mib to bytes
	requestedSize := size * 1024 * 1024
	// part size comes in bytes already
	currentSize := part.Size
	if size == 0 {
		// requested size is max, so calculate it
		// requested size is total disk size minus partition size
		// Also take into account the 2048 bytes for GPT backup header
		requestedSize = uint64(d.Size) - part.Start - (2048 * dev.SectorS)
	}
	if requestedSize <= currentSize {
		_ = d.Close()
		return fmt.Errorf("requested size is less than or equal to current partition size (requested %d sectors, current %d sectors)", requestedSize, currentSize)
	}

	// Calculate how many sectors we need to expand
	// expandSectors is the needed sectors to expand the disk
	// We need to take into account that its the size minus the partition current size, so only what we need to expand
	expandSectors := (requestedSize - currentSize) / dev.SectorS
	// Free size in the disk in sectors. We get the total size, then minus the end of the last partition
	freeSectorsInDisk := (uint64(d.Size)/dev.SectorS - part.End) - 1 // leave 1 sector at the end for backup GPT header
	// Check if there is enough space. Remember that all disks have a backup GPT header at the end, so we need to leave at least 1MiB free at the end + 1MiB alignment at the start
	if expandSectors > freeSectorsInDisk {
		_ = d.Close()
		return fmt.Errorf("not enough free space in disk: need %d MiB, available %d MiB", expandSectors*dev.SectorS/OneMiBInBytes, freeSectorsInDisk*dev.SectorS/OneMiBInBytes)
	}

	if size == 0 {
		part.End = dev.LastS - 1
	} else {
		part.End = part.Start + MiBToSectors(size, dev.SectorS) - 1
	}
	// We have to set Size to 0 so the GPT library recalculates it
	part.Size = 0
	err = d.Partition(gptTable)
	if err != nil {
		_ = d.Close()
		return err
	}
	// Now try to re-read partition table so kernel sees new partitions
	// This is on a best effort basis, as if the partition is in use, it will fail
	_ = d.ReReadPartitionTable()
	syscall.Sync()
	_, _ = console.Run("udevadm trigger && udevadm settle")

	// Now resize the underlying filesystem
	filesystem, err := DefaultFilesystemDetector.DetectFileSystemType(part, d)

	if err != nil {
		_ = d.Close()
		return fmt.Errorf("could not detect filesystem type: %w", err)
	}

	var device string
	partNumber := len(gptTable.Partitions)
	// NVMe devices have a different partition naming scheme
	if strings.Contains(dev.Device, "nvme") {
		device = fmt.Sprintf("%sp%d", dev.Device, partNumber)
	} else {
		device = fmt.Sprintf("%s%d", dev.Device, partNumber)
	}

	_ = d.Close()

	return DefaultGrowFsToMax.GrowFSToMax(device, filesystem)
}

func (dev *Disk) MatchPartitionFSLabel(label string) bool {
	for _, p := range dev.Parts {
		if p.FSLabel == label {
			return true
		}
	}
	return false
}

func (dev *Disk) MatchPartitionPLabel(label string) bool {
	for _, p := range dev.Parts {
		if p.PLabel == label {
			return true
		}
	}
	return false
}

func (mkfs MkfsCall) buildOptions() ([]string, error) {
	var opts []string

	linuxFS, _ := regexp.MatchString("ext[2-4]|xfs|btrfs|swap", mkfs.part.FileSystem)
	fatFS, _ := regexp.MatchString("fat|vfat", mkfs.part.FileSystem)

	switch {
	case linuxFS:
		if mkfs.part.FSLabel != "" {
			opts = append(opts, "-L")
			opts = append(opts, mkfs.part.FSLabel)
		}
		if mkfs.part.FileSystem == Btrfs {
			opts = append(opts, "-f")
		}
		if len(mkfs.customOpts) > 0 {
			opts = append(opts, mkfs.customOpts...)
		}
		opts = append(opts, mkfs.dev)
	case fatFS:
		if mkfs.part.FSLabel != "" {
			opts = append(opts, "-n")
			opts = append(opts, mkfs.part.FSLabel)
		}
		if len(mkfs.customOpts) > 0 {
			opts = append(opts, mkfs.customOpts...)
		}
		opts = append(opts, mkfs.dev)
	default:
		return []string{}, errors.New(fmt.Sprintf("Unsupported filesystem: %s", mkfs.part.FileSystem))
	}
	return opts, nil
}

func (mkfs MkfsCall) Apply(console Console) (string, error) {
	opts, err := mkfs.buildOptions()
	if err != nil {
		return "", err
	}

	var tool string

	if mkfs.part.FileSystem == "swap" {
		tool = "mkswap"
	} else if mkfs.part.FileSystem == "fat16" || mkfs.part.FileSystem == "fat32" || mkfs.part.FileSystem == "vfat" || mkfs.part.FileSystem == "fat" {
		tool = "mkfs.fat"
	} else {
		tool = fmt.Sprintf("mkfs.%s", mkfs.part.FileSystem)
	}
	_, err = exec.LookPath(tool)
	if err != nil {
		return "", fmt.Errorf("mkfs tool %s not found in PATH", tool)
	}
	command := fmt.Sprintf("%s %s", tool, strings.Join(opts[:], " "))
	return console.Run(command)
}

func MiBToSectors(size uint64, sectorSize uint64) uint64 {
	return size * 1048576 / sectorSize
}

func GetParts(d *disk.Disk) []Partition {
	parts := make([]Partition, 0)
	table, err := d.GetPartitionTable()
	if err != nil {
		return parts
	}
	for index, p := range table.GetPartitions() {
		if p == nil || p.GetStart() == 0 && p.GetSize() == 0 {
			continue
		}
		part := p.(*gpt.Partition)
		fs, err := DefaultFilesystemDetector.DetectFileSystemType(part, d)
		if err != nil {
			fs = "unknown"
		}
		parts = append(parts, Partition{
			Start:      part.Start,
			Size:       part.Size,
			End:        part.End,
			PLabel:     part.Name,
			FileSystem: fs,
			PartNumber: index + 1,
		})
	}
	return parts
}

// writePartitionTable writes the given partition table to the disk and updates the disk's Table field.
// This is a copy of diskfs's internal function to force a rewrite of the partition table.
// As the ReReadPartitionTable can fail if the disk is in use, we ignore the resulting error.
func writePartitionTable(d *disk.Disk, table partition.Table) error {
	rwBackingFile, err := d.Backend.Writable()
	if err != nil {
		return err
	}

	// fill in the uuid
	err = table.Write(rwBackingFile, d.Size)
	if err != nil {
		return fmt.Errorf("failed to write partition table: %v", err)
	}
	d.Table = table
	_ = d.ReReadPartitionTable()
	return nil
}

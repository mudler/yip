package plugins

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/partition/gpt"
	"github.com/gofrs/uuid"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs/v4"
)

const (
	extMagicOffset1          = 1080
	extMagicOffset2          = 1081
	extMagic1                = 0x53
	extMagic2                = 0xEF
	ext4ExtentFeatureOffset  = 1124
	ext3JournalFeatureOffset = 1084
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
)

type Disk struct {
	Device  string
	SectorS uint
	LastS   uint
	Parts   []Partition
	disk    *disk.Disk
}

type Partition struct {
	StartS     uint
	SizeS      uint
	PLabel     string
	FileSystem string
	FSLabel    string
}

type MkfsCall struct {
	part       Partition
	customOpts []string
	dev        string
}

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
		l.Debugf("Initializing disk with path %s", s.Layout.Device.InitDisk, s.Layout.Device.Path)
		d, err := diskfs.Open(s.Layout.Device.Path)
		if err != nil {
			l.Debugf("Disk initialization failed: %s", err)
			return err
		}
		defer func() {
			_ = d.Close()
		}()

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
			return err
		}
		l.Debugf("Initialized disk with path %s", s.Layout.Device.Path)
		syscall.Sync()
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

	changed := false
	l.Debugf("Checking for free space on device %s", dev.Device)
	if !dev.CheckDiskFreeSpaceMiB(32) {
		l.Warnf("Not enough unpartitioned space in disk to operate")
		return nil
	}

	l.Debugf("Going over the partition layout to create partitions on device %s", dev.Device)
	for _, part := range s.Layout.Parts {
		if part.FSLabel != "" {
			l.Debugf("Checking if partition with FSLabel: %s exists on device %s", part.FSLabel, dev.Device)
			if dev.MatchPartitionFSLabel(part.FSLabel) {
				l.Warnf("Partition with FSLabel: %s already exists, ignoring", part.FSLabel)
				continue
			}
		}
		if part.PLabel != "" {
			l.Debugf("Checking if partition with PLabel: %s exists on device %s", part.PLabel, dev.Device)
			if dev.MatchPartitionPLabel(part.PLabel) {
				l.Warnf("Partition with PLabel: %s already exists, ignoring", part.PLabel)
				continue
			}
		}

		if part.FileSystem == "" {
			part.FileSystem = "ext2"
		}

		l.Debugf("Creating partition with label %s, fslabel %s and fs %s on device %s", part.PLabel, part.FSLabel, part.FileSystem, dev.Device)
		output, err := dev.AddPartition(part.Size, part.PLabel, part.FSLabel, part.FileSystem, console)
		if err != nil {
			if output != "" {
				l.Debugf("Output from mkfs command: %s", output)
			}
			l.Error(err.Error())
			return err
		}
		changed = true
		l.Debugf("Created partition with label %s on device %s", part.FSLabel, dev.Device)
	}

	l.Debugf("Checking for layout expansion on device %s", dev.Device)
	if s.Layout.Expand != nil {
		if s.Layout.Expand.Size == 0 {
			l.Debug("Extending last partition to max space")
		} else {
			l.Debugf("Extending last partition to %d MiB", s.Layout.Expand.Size)
		}
		err := dev.ExpandLastPartition(s.Layout.Expand.Size)
		if err != nil {
			l.Error(err.Error())
			return err
		}
		l.Debugf("Extended last partition")
		changed = true
	}
	l.Debugf("Checking if we need to reload partition table on device %s: %v", dev.Device, changed)
	if changed {
		if err := dev.Reload(); err != nil {
			return err
		}
	}
	l.Debugf("All done with layout plugin for device %s", dev.Device)
	return nil
}

func FindDiskFromPath(path string, fs vfs.FS) (Disk, error) {
	rawPath, err := fs.RawPath(path)
	if err != nil {
		return Disk{}, fmt.Errorf("could not resolve raw path: %w", err)
	}
	d, err := diskfs.Open(rawPath)
	if err != nil {
		return Disk{}, fmt.Errorf("could not open disk: %w", err)
	}

	// Use d.LogicalBlocksize and d.Size directly
	return Disk{
		Device:  path,
		SectorS: uint(d.LogicalBlocksize),
		LastS:   uint(d.Size / d.LogicalBlocksize),
		disk:    d,
		Parts:   GetParts(d),
	}, nil
}

func FindDiskFromLabel(label string, fs vfs.FS) (Disk, error) {
	path, err := fs.RawPath(filepath.Join("/dev/disk/by-label", label))
	if err != nil {
		return Disk{}, fmt.Errorf("could not resolve disk by label: %w", err)
	}
	d, err := diskfs.Open(path)
	if err != nil {
		return Disk{}, fmt.Errorf("could not open disk: %w", err)
	}
	// Use d.LogicalBlocksize and d.Size directly
	return Disk{
		Device:  filepath.Join("/dev/disk/by-label", label),
		SectorS: uint(d.LogicalBlocksize),
		LastS:   uint(d.Size / d.LogicalBlocksize),
		disk:    d,
		Parts:   GetParts(d),
	}, nil
}

func (dev *Disk) Reload() error {
	dev.Parts = GetParts(dev.disk)
	return nil
}

func (dev *Disk) CheckDiskFreeSpaceMiB(minSpace uint) bool {
	freeS := dev.computeFreeSpace()
	minSec := MiBToSectors(minSpace, dev.SectorS)
	return freeS >= minSec
}

func (dev *Disk) computeFreeSpace() uint {
	if len(dev.Parts) > 0 {
		lastPart := dev.Parts[len(dev.Parts)-1]
		return dev.LastS - (lastPart.StartS + lastPart.SizeS - 1)
	}
	return dev.LastS - (OneMiBInBytes/dev.SectorS - 1)
}

func (dev *Disk) AddPartition(size uint, label, fsLabel, filesystem string, console Console) (string, error) {
	table, err := dev.disk.GetPartitionTable()
	if err != nil {
		return "", err
	}
	gptTable, ok := table.(*gpt.Table)
	if !ok {
		return "", errors.New("only GPT partition tables are supported")
	}

	var startS uint
	if len(dev.Parts) > 0 {
		last := dev.Parts[len(dev.Parts)-1]
		startS = last.StartS + last.SizeS
	}
	sizeS := MiBToSectors(size, dev.SectorS)
	if startS+sizeS > dev.LastS {
		availableMiB := ((dev.LastS - startS) * dev.SectorS) / OneMiBInBytes
		return "", fmt.Errorf("not enough free space in disk: required %d MiB, available %d MiB", size, availableMiB)
	}

	var fsType gpt.Type
	switch filesystem {
	case "ext2", "ext3", "ext4", "xfs", "btrfs":
		fsType = gpt.LinuxFilesystem
	case "fat16", "fat32", "vfat", "fat":
		fsType = gpt.EFISystemPartition
	case "swap":
		fsType = gpt.LinuxSwap
	default:
		return "", fmt.Errorf("unsupported filesystem type: %s", filesystem)
	}

	part := &gpt.Partition{
		Start: uint64(startS),
		End:   uint64(startS + sizeS - 1),
		Name:  label,
		Type:  fsType,
	}
	gptTable.Partitions = append(gptTable.Partitions, part)
	err = dev.disk.Partition(gptTable)
	if err != nil {
		return "", err
	}
	if err := dev.Reload(); err != nil {
		return "", err
	}

	mkfsPart := Partition{
		FileSystem: filesystem,
		PLabel:     label,
		FSLabel:    fsLabel,
	}

	mkfs := MkfsCall{part: mkfsPart, customOpts: []string{}, dev: dev.Device}
	return mkfs.Apply(console)

}

func (dev *Disk) ExpandLastPartition(size uint) error {
	if len(dev.Parts) == 0 {
		return errors.New("no partition to expand")
	}
	table, err := dev.disk.GetPartitionTable()
	if err != nil {
		return err
	}
	gptTable, ok := table.(*gpt.Table)
	if !ok {
		return errors.New("only GPT partition tables are supported")
	}
	lastIdx := len(gptTable.Partitions) - 1
	if lastIdx < 0 {
		return errors.New("no partition to expand")
	}
	part := gptTable.Partitions[lastIdx]
	if part == nil {
		return errors.New("last partition is nil")
	}
	// Check if the partition is swap as we cannot expand swap partitions
	if part.Type == gpt.LinuxSwap {
		return errors.New("swap resizing is not supported")
	}
	// Check if requested size is less than actual size
	currentSize := part.End - part.Start + 1
	var requestedSize uint64
	// Setting Size to 0 tells the GPT library to recalculate the partition size based on Start and End.
	if size == 0 {
		requestedSize = uint64(dev.LastS) - part.Start
	} else {
		requestedSize = uint64(MiBToSectors(size, dev.SectorS))
	}
	if requestedSize <= currentSize {
		return errors.New("requested size is less than or equal to current partition size")
	}

	// Check if there is enough space to expand in the disk
	availableSpace := uint64(dev.LastS) - part.End - 1
	if requestedSize-currentSize > availableSpace {
		availableMiB := (availableSpace * uint64(dev.SectorS)) / OneMiBInBytes
		return fmt.Errorf("not enough space to expand the partition (Available: %d MiB)", availableMiB)
	}
	if size == 0 {
		part.End = uint64(dev.LastS - 1)
	} else {
		part.End = part.Start + uint64(MiBToSectors(size, dev.SectorS)) - 1
	}
	part.Size = 0
	err = dev.disk.Partition(gptTable)
	if err != nil {
		return err
	}
	return dev.Reload()
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
		if mkfs.part.FileSystem == "btrfs" {
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
	} else {
		tool = fmt.Sprintf("mkfs.%s", mkfs.part.FileSystem)
	}

	command := fmt.Sprintf("%s %s", tool, strings.Join(opts[:], " "))
	return console.Run(command)
}

func MiBToSectors(size uint, sectorSize uint) uint {
	return size * 1048576 / sectorSize
}

func GetParts(d *disk.Disk) []Partition {
	parts := make([]Partition, 0)
	table, err := d.GetPartitionTable()
	if err != nil {
		return parts
	}
	for _, p := range table.GetPartitions() {
		if p == nil || p.GetStart() == 0 && p.GetSize() == 0 {
			continue
		}
		part := p.(*gpt.Partition)
		fs, err := DetectFileSystemType(part, d)
		if err != nil {
			fs = "unknown"
		}
		parts = append(parts, Partition{
			StartS:     uint(p.GetStart()),
			SizeS:      uint(p.GetSize()),
			PLabel:     part.Name,
			FileSystem: fs,
		})
	}
	return parts
}

// DetectFileSystemType tries to identify the filesystem by reading magic numbers.
func DetectFileSystemType(part *gpt.Partition, d *disk.Disk) (string, error) {
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
		if buf[ext4ExtentFeatureOffset]&0x40 != 0 {
			return "ext4", nil
		}
		// Check for ext3: has_journal feature (bit 0x4) in feature_compat at 1084
		if buf[ext3JournalFeatureOffset]&0x4 != 0 {
			return "ext3", nil
		}
		// Otherwise, assume ext2
		return "ext2", nil
	}

	// FAT16: "FAT" at offset 54 (FAT12/16)
	if len(buf) > fat16MagicOffset2 && bytes.Equal(buf[fat16MagicOffset1:fat16MagicOffset2], []byte(fat16Magic)) {
		return "fat", nil
	}
	// FAT32: "FAT32   " at offset 82 (FAT32, 8 bytes with spaces)
	// Be more lax with FAT32 detection due to variations in the magic string or extra characters
	if len(buf) > fat32MagicOffset2 && bytes.Contains(buf[fat32MagicOffset1:fat32MagicOffset2], []byte(fat32Magic)) {
		return "fat", nil
	}

	// btrfs: "_BHRfS_M" at offset 0x40
	if len(buf) > 0x47 && bytes.Equal(buf[btrfsMagicOffset1:btrfsMagicOffset2], []byte(btrfsMagic)) {
		return "btrfs", nil
	}

	// xfs: "XFSB" at offset 0
	if len(buf) > 4 && bytes.Equal(buf[xfsMagicOffset1:xfsMagicOffset2], []byte(xfsMagic)) {
		return "xfs", nil
	}

	// swap: "SWAPSPACE2" at end of partition
	swapSig := []byte(swapMagicSignature)
	endOffset := int64((part.End+1)*uint64(sectorSize)) - int64(len(swapSig))
	swapBuf := make([]byte, len(swapSig))
	_, err = d.Backend.ReadAt(swapBuf, endOffset)
	if err == nil && bytes.Equal(swapBuf, swapSig) {
		return "swap", nil
	}
	return "", errors.New("unknown filesystem")
}

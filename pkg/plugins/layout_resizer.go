package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

const (
	// BLKGETSIZE64 returns u64 device size in bytes when called on a block device fd.
	BLKGETSIZE64 = 0x80081272
	// EXT4_IOC_RESIZE_FS precomputed value for 64-bit Linux (x86_64, arm64, riscv64, ppc64le)
	// Otherwise make a IOW('f', 16, __u64) -> 0x40086610
	// https://github.com/torvalds/linux/blob/master/include/uapi/linux/ext4.h#L26
	EXT4_IOC_RESIZE_FS = 0x40086610

	// XFS_IOC_FSGROWFSDATA = _IOW('X', 110, struct xfs_growfs_data) -> size=16 -> 0x4010586e
	// https://github.com/torvalds/linux/blob/master/fs/xfs/libxfs/xfs_fs.h#L1059
	XFS_IOC_FSGROWFSDATA = 0x4010586e

	// BTRFS_IOC_RESIZE = _IOW(0x94, 3, struct btrfs_ioctl_vol_args) -> size=4096 -> 0x50009403
	// https://github.com/torvalds/linux/blob/master/include/uapi/linux/btrfs.h#L1106
	BTRFS_IOC_RESIZE = 0x50009403
)

type xfsGrowfsData struct {
	NewBlocks uint64 // absolute data section size, in FS blocks
	ImaxPct   uint32 // 0 => unchanged
	Pad       uint32
}

// Kernel UAPI: struct btrfs_ioctl_vol_args { __s64 fd; char name[BTRFS_PATH_NAME_MAX+1]; }
// BTRFS_PATH_NAME_MAX is 4087, so sizeof(struct) = 8 + 4088 = 4096.
type btrfsIoctlVolArgs struct {
	Fd   int64
	Name [4096 - 8]byte // 4088 bytes
}

// GrowFsToMaxInterface defines an interface for growing filesystems to max size.
type GrowFsToMaxInterface interface {
	GrowFSToMax(device string, filesystem string) error
}

// RealGrowFsToMax is the real implementation of GrowFsToMaxInterface.
type RealGrowFsToMax struct{}

// DefaultGrowFsToMax is the default instance of GrowFsToMaxInterface.
var DefaultGrowFsToMax GrowFsToMaxInterface = &RealGrowFsToMax{}

// GrowFSToMax grows the filesystem on the given block device path
// to the maximum available space in the partition.
// fsType: "ext4" (works for ext3/ext2 via ext4 driver), "xfs", "btrfs".
//
// Growing needs the filesystem mounted, and that mount is unshared into a
// private mount namespace so it is not visible to the rest of the system. A
// mount namespace belongs to an OS thread, not to a goroutine, so this runs on
// a thread of its own. See runOnDedicatedThread.
func (r *RealGrowFsToMax) GrowFSToMax(devicePath, fsType string) error {
	switch fsType {
	case Ext4, Ext3, Ext2:
		return runOnDedicatedThread(func() error { return growExtFSToMax(devicePath) })
	case Xfs:
		return runOnDedicatedThread(func() error { return growXfsToMax(devicePath) })
	case Btrfs:
		return runOnDedicatedThread(func() error { return growBtrfsToMax(devicePath) })
	default:
		return fmt.Errorf("unsupported fsType %q; expected ext4/xfs/btrfs", fsType)
	}
}

// runOnDedicatedThread runs fn on an OS thread that no other goroutine can be
// scheduled onto, and waits for it.
//
// unshare(CLONE_NEWNS) changes the mount namespace of the calling thread. Go
// gives no goroutine a thread of its own, so without this an unshare deep
// inside a library call leaves a thread behind whose view of the mount tree is
// frozen at that moment, and the runtime then hands that thread to whatever
// goroutine asks next. Callers that mount something afterwards, from another
// thread, do not see it there: the read-only image underneath shows through
// instead, and which callers are hit varies from run to run.
//
// Locking the goroutine to its thread and never unlocking it makes the runtime
// destroy the thread when the goroutine returns, so the namespace dies with it.
func runOnDedicatedThread(fn func() error) error {
	done := make(chan error, 1)
	go func() {
		// Deliberately not unlocked: an unlocked thread goes back into the
		// pool carrying whatever namespace fn left on it.
		runtime.LockOSThread()
		done <- fn()
	}()
	return <-done
}

// GrowExtFSToMax grows an ext4/ext3/ext2 filesystem on the given block device path
// to the maximum available space in the partition
// fstype should generally be ext4 as it also deals with ext3/ext2.
func growExtFSToMax(devicePath string) error {
	if devicePath == "" {
		return errors.New("empty device path")
	}

	// Open block device to read its byte size
	dev, err := os.OpenFile(devicePath, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("open device: %w", err)
	}
	defer func() { _ = dev.Close() }()

	devBytes, err := blkGetSize64(int(dev.Fd()))
	if err != nil {
		return fmt.Errorf("BLKGETSIZE64: %w", err)
	}
	if devBytes == 0 {
		return fmt.Errorf("device reports 0 bytes")
	}

	// Ephemeral mount so ioctl hits the filesystem
	// we dont care if its ext3/ext2 as ext4 driver handles them too
	// in fact there is been years since ext3/ext2 had their own drivers in the kernel
	mp, cleanup, err := ephemeralMount(devicePath, Ext4)
	if err != nil {
		return fmt.Errorf("mount ext4: %w", err)
	}
	defer cleanup()

	// Get filesystem block size (for ext4 ioctl: units = blocks)
	fsBlock, err := fsBlockSizeFromStatfs(mp)
	if err != nil {
		return fmt.Errorf("statfs: %w", err)
	}
	newBlocks := devBytes / fsBlock
	if newBlocks == 0 {
		return fmt.Errorf("computed 0 target blocks")
	}

	// Issue ioctl on an fd inside the mounted FS
	df, err := os.Open(mp)
	if err != nil {
		return fmt.Errorf("open mountpoint: %w", err)
	}
	defer func() { _ = df.Close() }()

	_, _, errno := unix.Syscall(unix.SYS_IOCTL,
		df.Fd(),
		EXT4_IOC_RESIZE_FS,
		uintptr(unsafe.Pointer(&newBlocks)),
	)
	if errno != 0 {
		return fmt.Errorf("EXT4_IOC_RESIZE_FS failed: %w", errno)
	}
	return nil
}

func growXfsToMax(devicePath string) error {
	if devicePath == "" {
		return errors.New("empty device path")
	}

	// Open device to confirm it exists & size (optional but helpful)
	dev, err := os.OpenFile(devicePath, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("open device: %w", err)
	}
	defer func() { _ = dev.Close() }()

	devBytes, err := blkGetSize64(int(dev.Fd()))
	if err != nil {
		return fmt.Errorf("BLKGETSIZE64: %w", err)
	}
	if devBytes == 0 {
		return fmt.Errorf("device reports 0 bytes")
	}

	mp, cleanup, err := ephemeralMount(devicePath, Xfs)
	if err != nil {
		return fmt.Errorf("mount xfs: %w", err)
	}
	defer cleanup()

	fsBlock, err := fsBlockSizeFromStatfs(mp)
	if err != nil {
		return fmt.Errorf("statfs: %w", err)
	}

	args := xfsGrowfsData{
		NewBlocks: devBytes / fsBlock,
		ImaxPct:   0,
	}
	if args.NewBlocks == 0 {
		return fmt.Errorf("computed 0 target blocks")
	}

	df, err := os.Open(mp)
	if err != nil {
		return fmt.Errorf("open mountpoint: %w", err)
	}
	defer func() { _ = df.Close() }()

	_, _, errno := unix.Syscall(unix.SYS_IOCTL,
		df.Fd(),
		XFS_IOC_FSGROWFSDATA,
		uintptr(unsafe.Pointer(&args)),
	)
	if errno != 0 {
		return fmt.Errorf("XFS_IOC_FSGROWFSDATA failed: %w", errno)
	}
	return nil
}

func growBtrfsToMax(devicePath string) error {
	if devicePath == "" {
		return errors.New("empty device path")
	}

	// Optional sanity open; not required by ioctl itself
	dev, err := os.OpenFile(devicePath, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("open device: %w", err)
	}
	_ = dev.Close()

	mp, cleanup, err := ephemeralMount(devicePath, Btrfs)
	if err != nil {
		return fmt.Errorf("mount btrfs: %w", err)
	}
	defer cleanup()

	df, err := os.Open(mp)
	if err != nil {
		return fmt.Errorf("open mountpoint: %w", err)
	}
	defer func() { _ = df.Close() }()

	var args btrfsIoctlVolArgs
	args.Fd = -1
	copy(args.Name[:], "max\x00") // "max" -> grow to max for single-device FS

	_, _, errno := unix.Syscall(unix.SYS_IOCTL,
		df.Fd(),
		BTRFS_IOC_RESIZE,
		uintptr(unsafe.Pointer(&args)),
	)
	if errno != 0 {
		return fmt.Errorf("BTRFS_IOC_RESIZE failed: %w", errno)
	}
	return nil
}

func blkGetSize64(fd int) (uint64, error) {
	var size uint64
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), BLKGETSIZE64, uintptr(unsafe.Pointer(&size)))
	if errno != 0 {
		return 0, errno
	}
	return size, nil
}

func fsBlockSizeFromStatfs(path string) (uint64, error) {
	var s unix.Statfs_t
	if err := unix.Statfs(path, &s); err != nil {
		return 0, err
	}
	if s.Bsize <= 0 {
		return 0, fmt.Errorf("invalid fs block size: %d", s.Bsize)
	}
	return uint64(s.Bsize), nil
}

type cleanupFn func()

func ephemeralMount(dev, fstype string) (mountpoint string, cleanup cleanupFn, err error) {
	// Use a per-process temp dir, keep it tidy.
	tmpBase := os.TempDir()
	mp := filepath.Join(tmpBase, fmt.Sprintf("fs-grow-%d", os.Getpid()))
	if err := os.MkdirAll(mp, 0o755); err != nil {
		return "", nil, err
	}

	// Try to isolate the mount (best-effort): private mount namespace on Linux.
	// If unshare fails (e.g., old kernels or lacking caps), we still proceed
	// because we immediately unmount afterwards.
	//
	// Callers must already be on a dedicated thread, see runOnDedicatedThread:
	// the unshare below applies to the thread, not to the goroutine.
	if err := unix.Unshare(unix.CLONE_NEWNS); err == nil {
		// Only inside the new namespace. In the shared one this turns every
		// mount on the machine private and breaks propagation for everybody.
		_ = unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, "")
	}

	if err := unix.Mount(dev, mp, fstype, 0, ""); err != nil {
		_ = os.RemoveAll(mp)
		return "", nil, err
	}

	cleanup = func() {
		// Ensure unmount even if busy: try lazy umount as fallback.
		if err := unix.Unmount(mp, 0); err != nil {
			_ = unix.Unmount(mp, unix.MNT_DETACH)
		}
		_ = os.RemoveAll(mp)
	}

	// No finalizer: it would run on an arbitrary thread, in the wrong mount
	// namespace, and unmount nothing. Every caller defers cleanup already.

	return mp, cleanup, nil
}

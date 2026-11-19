package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeSysfs builds the parts of /sys that the block device helpers read, under
// a temporary root. It lays out the same symlinks the kernel does: every
// /sys/class/block/<name> points into /sys/devices, a partition lives in the
// directory of its disk, and a device-mapper device has a dm/ directory and
// one slaves/ entry per device it sits on.
type fakeSysfs struct {
	t    *testing.T
	root string
}

func newFakeSysfs(t *testing.T) *fakeSysfs {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sys/class/block"), 0o755); err != nil {
		t.Fatal(err)
	}
	return &fakeSysfs{t: t, root: root}
}

// link points /sys/class/block/<name> at the device directory devDir, given
// relative to /sys/devices.
func (f *fakeSysfs) link(name, devDir string) string {
	f.t.Helper()
	target := filepath.Join(f.root, "sys/devices", devDir)
	if err := os.MkdirAll(target, 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(f.root, "sys/class/block", name)); err != nil {
		f.t.Fatal(err)
	}
	return target
}

func (f *fakeSysfs) disk(name, parentDir string) {
	f.link(name, filepath.Join(parentDir, "block", name))
}

func (f *fakeSysfs) partition(disk, name, parentDir string) {
	f.link(name, filepath.Join(parentDir, "block", disk, name))
}

func (f *fakeSysfs) mapper(name string, slaves ...string) {
	f.t.Helper()
	dir := f.link(name, filepath.Join("virtual/block", name))
	for _, sub := range []string{"dm", "slaves"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			f.t.Fatal(err)
		}
	}
	for _, slave := range slaves {
		target, err := filepath.EvalSymlinks(filepath.Join(f.root, "sys/class/block", slave))
		if err != nil {
			f.t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(dir, "slaves", slave)); err != nil {
			f.t.Fatal(err)
		}
	}
}

func TestBackingDeviceAndParentDisk(t *testing.T) {
	f := newFakeSysfs(t)
	pci := "pci0000:00/0000:00:1f.2/ata1/host0/target0:0:0/0:0:0:0"
	nvme := "pci0000:00/0000:00:01.0/nvme/nvme0"
	f.disk("sda", pci)
	f.partition("sda", "sda5", pci)
	f.disk("nvme0n1", nvme)
	f.partition("nvme0n1", "nvme0n1p3", nvme)
	f.disk("sdb", pci)
	f.partition("sdb", "sdb1", pci)
	// LUKS on a partition, as Kairos sets up an encrypted COS_PERSISTENT.
	f.mapper("dm-0", "sda5")
	// LUKS on top of an LVM volume on a partition.
	f.mapper("dm-1", "nvme0n1p3")
	f.mapper("dm-2", "dm-1")
	// A mapping that spans two devices has no single partition to grow.
	f.mapper("dm-3", "sda5", "sdb1")

	tests := []struct {
		name        string
		dev         string
		wantBacking string
		wantMapper  bool
		wantDisk    string
		wantErr     bool
	}{
		{name: "plain partition", dev: "/dev/sda5", wantBacking: "/dev/sda5", wantDisk: "/dev/sda"},
		{name: "nvme partition", dev: "/dev/nvme0n1p3", wantBacking: "/dev/nvme0n1p3", wantDisk: "/dev/nvme0n1"},
		{name: "luks mapping on a partition", dev: "/dev/dm-0", wantBacking: "/dev/sda5", wantMapper: true, wantDisk: "/dev/sda"},
		{name: "stacked mappings", dev: "/dev/dm-2", wantBacking: "/dev/nvme0n1p3", wantMapper: true, wantDisk: "/dev/nvme0n1"},
		{name: "mapping on two devices", dev: "/dev/dm-3", wantMapper: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backing, isMapper, err := backingDevice(f.root, tt.dev)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("backingDevice(%s) = %s, want an error", tt.dev, backing)
				}
				return
			}
			if err != nil {
				t.Fatalf("backingDevice(%s): %v", tt.dev, err)
			}
			if backing != tt.wantBacking || isMapper != tt.wantMapper {
				t.Fatalf("backingDevice(%s) = %s, %v; want %s, %v", tt.dev, backing, isMapper, tt.wantBacking, tt.wantMapper)
			}
			disk, err := parentDiskFromBlockDev(f.root, backing)
			if err != nil {
				t.Fatalf("parentDiskFromBlockDev(%s): %v", backing, err)
			}
			if disk != tt.wantDisk {
				t.Fatalf("parentDiskFromBlockDev(%s) = %s, want %s", backing, disk, tt.wantDisk)
			}
		})
	}
}

// A device-mapper device has no disk above it in sysfs: its directory is
// /sys/devices/virtual/block/dm-0, so the old code took "block" as the disk.
// That is the error an encrypted persistent partition logged on every boot.
func TestParentDiskOfMapperFails(t *testing.T) {
	f := newFakeSysfs(t)
	f.disk("sda", "pci0000:00")
	f.partition("sda", "sda5", "pci0000:00")
	f.mapper("dm-0", "sda5")
	if disk, err := parentDiskFromBlockDev(f.root, "/dev/dm-0"); err == nil {
		t.Fatalf("parentDiskFromBlockDev(/dev/dm-0) = %s, want an error", disk)
	}
}

func TestBackingDeviceOfImageFile(t *testing.T) {
	f := newFakeSysfs(t)
	backing, isMapper, err := backingDevice(f.root, "/tmp/disk.img")
	if err != nil || isMapper || backing != "/tmp/disk.img" {
		t.Fatalf("backingDevice(/tmp/disk.img) = %s, %v, %v; want it unchanged", backing, isMapper, err)
	}
}

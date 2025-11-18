package plugins_test

import (
	"fmt"
	"io"

	"github.com/diskfs/go-diskfs"
	fileBackend "github.com/diskfs/go-diskfs/backend/file"
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/partition/gpt"
	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	console "github.com/mudler/yip/tests/console"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sanity-io/litter"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v4"
	"github.com/twpayne/go-vfs/v4/vfst"
)

// This are the reserved sectors in a GPT partition table (2048 sectors of 512 bytes)
const reservedSectorsInBytes = uint64(2048 * diskfs.SectorSize512)

// This tests run against a real disk image file created in a temp folder
// The mkfs calls are the ones mocked as we cannot run mkfs against a file image partition without
// having to mount it into a loop device and so on, but on a real device these calls would run as expected

// MockFilesystemDetector implements FilesystemDetector for testing.
type MockFilesystemDetector struct {
	DetectFunc func(part *gpt.Partition, d *disk.Disk) (string, error)
}

func (m MockFilesystemDetector) DetectFileSystemType(part *gpt.Partition, d *disk.Disk) (string, error) {
	if m.DetectFunc != nil {
		return m.DetectFunc(part, d)
	}
	fmt.Print("MockFilesystemDetector called\n")
	return "mockfs", nil
}

// MockGrowFSToMax implements GrowFSToMax for testing.
type MockGrowFSToMax struct {
	GrowFunc func(device string, filesystem string) error
}

func (m MockGrowFSToMax) GrowFSToMax(device string, filesystem string) error {
	if m.GrowFunc != nil {
		return m.GrowFunc(device, filesystem)
	}
	fmt.Print("MockGrowFSToMax called\n")
	return nil
}

var _ = Describe("Layout", Label("layout"), func() {
	var deviceLabel string
	var devicePath = "/test.img"
	var rawDevicePath string
	var label = "FAKELABEL"
	var err error
	var fs vfs.FS
	var cleanup func()

	BeforeEach(func() {
		fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).Should(BeNil())
		Expect(fs.Mkdir("/dev/", 0755)).Should(BeNil())
		Expect(fs.Mkdir("/dev/disk", 0755)).Should(BeNil())
		Expect(fs.Mkdir("/dev/disk/by-label", 0755)).Should(BeNil())
		// Create a temp disk image
		rawDevicePath, err = fs.RawPath("/test.img")
		Expect(err).Should(BeNil())
		fileDisk, err := fileBackend.CreateFromPath(rawDevicePath, 1*1024*1024*1024+int64(reservedSectorsInBytes)) // 1GiB + reserved sectors at the start
		Expect(err).To(BeNil())
		Expect(fileDisk.Close()).ToNot(HaveOccurred())
		// create initial gpt table with empty partitions
		d, err := diskfs.Open(rawDevicePath)
		Expect(err).To(BeNil())
		table := &gpt.Table{
			ProtectiveMBR:      true,
			LogicalSectorSize:  int(d.LogicalBlocksize),
			PhysicalSectorSize: int(d.PhysicalBlocksize),
		}
		err = d.Partition(table)
		Expect(err).To(BeNil())
		Expect(d.Close()).ToNot(HaveOccurred())
		DefaultFilesystemDetector = MockFilesystemDetector{func(part *gpt.Partition, d *disk.Disk) (string, error) {
			return "ext4", nil
		}}
		DefaultGrowFsToMax = MockGrowFSToMax{func(device string, filesystem string) error {
			return nil
		}}
	})
	AfterEach(func() {
		// clean up
		cleanup()
	})
	Context("creating", func() {

		l := logrus.New()
		l.SetLevel(logrus.DebugLevel)
		l.SetOutput(io.Discard)

		It("Fails to find device by path", func() {
			testConsole := console.New()
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Path: "/not/existing/device"},
					Parts:  []schema.Partition{{PLabel: label, Size: 100}},
				},
			}, fs, testConsole)
			Expect(err).To(HaveOccurred())
		})
		It("Fails to find device by label", func() {
			testConsole := console.New()
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Label: "WEIRDLABELIHOPEITDOESNTEXISTS"},
					Parts:  []schema.Partition{{PLabel: label, Size: 100}},
				},
			}, fs, testConsole)
			Expect(err).To(HaveOccurred())
		})
		It("Adds a new partition by path", func() {
			testConsole := console.New()
			testConsole.AddCmd(console.CmdMock{Cmd: "udevadm trigger && udevadm settle"})
			// Note that the mkfs.ext2 call goes to device + partition number, but since we mock it,
			// we just check that the call is made, so we use devicePath directly with a 1 at the end to mock it
			// even if this is a image file
			testConsole.AddCmd(console.CmdMock{Cmd: fmt.Sprintf("mkfs.ext2 %s1", devicePath)})
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Path: devicePath},
					Parts:  []schema.Partition{{PLabel: label, Size: 100}},
				},
			}, fs, testConsole)
			Expect(err).Should(BeNil())

			disk, err := fileBackend.OpenFromPath(rawDevicePath, true)
			defer disk.Close()
			table, err := gpt.Read(disk, int(diskfs.SectorSize512), int(diskfs.SectorSize512))
			Expect(err).ToNot(HaveOccurred())
			// check that its type GPT
			Expect(table.Type()).To(Equal("gpt"))
			Expect(table.Partitions).To(HaveLen(1))
			deviceLabel = table.Partitions[0].Name
			Expect(deviceLabel).To(Equal("FAKELABEL"), litter.Sdump(table.Partitions))
			Expect(table.Partitions[0].Size).To(Equal(uint64(100*1024*1024)-reservedSectorsInBytes), litter.Sdump(table))
		})
		It("Adds a new partition by path with fsLabel", func() {
			testConsole := console.New()
			testConsole.AddCmd(console.CmdMock{Cmd: "udevadm trigger && udevadm settle"})
			testConsole.AddCmd(console.CmdMock{Cmd: fmt.Sprintf("mkfs.ext2 -L FSLABEL %s1", devicePath)})
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Path: devicePath},
					Parts:  []schema.Partition{{PLabel: label, FSLabel: "FSLABEL", Size: 100}},
				},
			}, fs, testConsole)
			Expect(err).Should(BeNil())
			disk, err := fileBackend.OpenFromPath(rawDevicePath, true)
			defer disk.Close()
			table, err := gpt.Read(disk, int(diskfs.SectorSize512), int(diskfs.SectorSize512))
			Expect(err).ToNot(HaveOccurred())
			// check that its type GPT
			Expect(table.Type()).To(Equal("gpt"))
			Expect(table.Partitions).To(HaveLen(1))
			deviceLabel = table.Partitions[0].Name
			Expect(deviceLabel).To(Equal("FAKELABEL"))
			Expect(table.Partitions[0].Size).To(Equal(uint64(100*1024*1024) - reservedSectorsInBytes))
		})
		It("Adds a new partition by label", func() {
			Expect(fs.Symlink(devicePath, "/dev/disk/by-label/SOMELABEL")).Should(BeNil())
			testConsole := console.New()
			testConsole.AddCmd(console.CmdMock{Cmd: "udevadm trigger && udevadm settle"})
			testConsole.AddCmd(console.CmdMock{Cmd: fmt.Sprintf("mkfs.ext2 /dev/disk/by-label/SOMELABEL")})
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Label: "SOMELABEL"},
					Parts:  []schema.Partition{{PLabel: "PLABEL", Size: 100}},
				},
			}, fs, testConsole)
			Expect(err).Should(BeNil())
		})
		It("Adds a new partition by label with fsLabel", func() {
			Expect(fs.Symlink(devicePath, "/dev/disk/by-label/SOMELABEL")).Should(BeNil())
			testConsole := console.New()
			testConsole.AddCmd(console.CmdMock{Cmd: "udevadm trigger && udevadm settle"})
			testConsole.AddCmd(console.CmdMock{Cmd: fmt.Sprintf("mkfs.ext2 -L MYLABEL /dev/disk/by-label/SOMELABEL")})
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Label: "SOMELABEL"},
					Parts:  []schema.Partition{{FSLabel: "MYLABEL", Size: 100}},
				},
			}, fs, testConsole)
			Expect(err).Should(BeNil())
		})
		It("Fails to add a partition of 1025MiB, there are only 1024MiB available", func() {
			testConsole := console.New()
			testConsole.AddCmd(console.CmdMock{Cmd: "udevadm trigger && udevadm settle"})
			testConsole.AddCmd(console.CmdMock{Cmd: fmt.Sprintf("mkfs.ext2 -L %s %s", label, devicePath)})
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Path: devicePath},
					Parts:  []schema.Partition{{FSLabel: label, Size: 1025}},
				},
			}, fs, testConsole)
			Expect(err).To(HaveOccurred())
		})
		It("Ignores an already existing partition", func() {
			testConsole := console.New()
			testConsole.AddCmd(console.CmdMock{Cmd: "udevadm trigger && udevadm settle"})
			testConsole.AddCmd(console.CmdMock{Cmd: fmt.Sprintf("mkfs.ext2 %s1", devicePath)})
			testConsole.AddCmd(console.CmdMock{Cmd: "udevadm trigger && udevadm settle"})
			testConsole.AddCmd(console.CmdMock{Cmd: fmt.Sprintf("mkfs.ext2 %s1", devicePath)})
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Path: devicePath},
					Parts:  []schema.Partition{{PLabel: label, Size: 100}},
				},
			}, fs, testConsole)
			Expect(err).ToNot(HaveOccurred())
			// Now we run again with same partition and it should fail to add it
			err = Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Path: devicePath},
					Parts:  []schema.Partition{{PLabel: label, Size: 100}},
				},
			}, fs, testConsole)
			Expect(err).ToNot(HaveOccurred())

			// Now lets check if this is true and there is only a single partition
			disk, err := fileBackend.OpenFromPath(rawDevicePath, true)
			defer disk.Close()
			table, err := gpt.Read(disk, int(diskfs.SectorSize512), int(diskfs.SectorSize512))
			Expect(err).ToNot(HaveOccurred())
			// check that its type GPT
			Expect(table.Type()).To(Equal("gpt"))
			Expect(table.Partitions).To(HaveLen(1))
			deviceLabel = table.Partitions[0].Name
			Expect(deviceLabel).To(Equal("FAKELABEL"))
			Expect(table.Partitions[0].Size).To(Equal(uint64(100*1024*1024) - reservedSectorsInBytes))

		})
		It("Fails to expand last partition, it can't shrink a partition", func() {
			testConsole := console.New()
			testConsole.AddCmd(console.CmdMock{Cmd: "udevadm trigger && udevadm settle"})
			testConsole.AddCmd(console.CmdMock{Cmd: fmt.Sprintf("mkfs.ext2 %s1", devicePath)})
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Path: devicePath},
					Parts:  []schema.Partition{{PLabel: label, Size: 512}},
				},
			}, fs, testConsole)
			Expect(err).ToNot(HaveOccurred())

			// Now we try to shrink it
			err = Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Path: devicePath},
					Expand: &schema.Expand{Size: 256},
				},
			}, fs, testConsole)
			Expect(err).To(HaveOccurred())
		})
		It("Expands last partition", func() {
			testConsole := console.New()
			testConsole.AddCmd(console.CmdMock{Cmd: "udevadm trigger && udevadm settle"})
			testConsole.AddCmd(console.CmdMock{Cmd: fmt.Sprintf("mkfs.ext2 %s1", devicePath)})
			testConsole.AddCmd(console.CmdMock{Cmd: "udevadm trigger && udevadm settle"})
			testConsole.AddCmd(console.CmdMock{Cmd: fmt.Sprintf("mkfs.ext2 %s1", devicePath)})
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Path: devicePath},
					Parts:  []schema.Partition{{PLabel: label, Size: 512}},
				},
			}, fs, testConsole)
			Expect(err).Should(BeNil())

			// check that indeed its 512MiB
			disk1, err := fileBackend.OpenFromPath(rawDevicePath, true)
			defer disk1.Close()
			table1, err := gpt.Read(disk1, int(diskfs.SectorSize512), int(diskfs.SectorSize512))
			Expect(err).ToNot(HaveOccurred())
			// check that its type GPT
			Expect(table1.Type()).To(Equal("gpt"))
			Expect(table1.Partitions).To(HaveLen(1))
			deviceLabel = table1.Partitions[0].Name
			Expect(deviceLabel).To(Equal("FAKELABEL"))
			Expect(table1.Partitions[0].Size).To(Equal(uint64(512*1024*1024) - reservedSectorsInBytes))
			// Now expand it
			err = Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Path: devicePath},
					Expand: &schema.Expand{Size: 1024},
				},
			}, fs, testConsole)
			Expect(err).Should(BeNil())
			// Now check if the partition size is now 1024MiB
			disk2, err := fileBackend.OpenFromPath(rawDevicePath, true)
			defer disk2.Close()
			table2, err := gpt.Read(disk2, int(diskfs.SectorSize512), int(diskfs.SectorSize512))
			Expect(err).ToNot(HaveOccurred())
			// check that its type GPT
			Expect(table2.Type()).To(Equal("gpt"))
			Expect(table2.Partitions).To(HaveLen(1))
			deviceLabel = table2.Partitions[0].Name
			Expect(deviceLabel).To(Equal("FAKELABEL"))
			Expect(table2.Partitions[0].Size).To(Equal(uint64(1024 * 1024 * 1024)))

		})
		It("Expands last partition to take all space", func() {
			DefaultFilesystemDetector = MockFilesystemDetector{func(part *gpt.Partition, d *disk.Disk) (string, error) {
				return "ext4", nil
			}}
			DefaultGrowFsToMax = MockGrowFSToMax{func(device string, filesystem string) error {
				return nil
			}}
			testConsole := console.New()
			testConsole.AddCmd(console.CmdMock{Cmd: "udevadm trigger && udevadm settle"})
			testConsole.AddCmd(console.CmdMock{Cmd: fmt.Sprintf("mkfs.ext2 %s1", devicePath)})
			testConsole.AddCmd(console.CmdMock{Cmd: "udevadm trigger && udevadm settle"})
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Path: devicePath},
					Parts:  []schema.Partition{{PLabel: label, Size: 512}},
				},
			}, fs, testConsole)
			Expect(err).Should(BeNil())

			// check that indeed its 512MiB
			disk1, err := fileBackend.OpenFromPath(rawDevicePath, true)
			defer disk1.Close()
			table1, err := gpt.Read(disk1, int(diskfs.SectorSize512), int(diskfs.SectorSize512))
			Expect(err).ToNot(HaveOccurred())
			// check that its type GPT
			Expect(table1.Type()).To(Equal("gpt"))
			Expect(table1.Partitions).To(HaveLen(1))
			deviceLabel = table1.Partitions[0].Name
			Expect(deviceLabel).To(Equal("FAKELABEL"))
			Expect(table1.Partitions[0].Size).To(Equal(uint64(512*1024*1024) - reservedSectorsInBytes))

			// Now expand it
			By("expanding to max size")
			err = Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Path: devicePath},
					Expand: &schema.Expand{},
				},
			}, fs, testConsole)
			Expect(err).Should(BeNil())

			// Now check if the partition size is now 1024MiB
			disk2, err := fileBackend.OpenFromPath(rawDevicePath, true)
			defer disk2.Close()
			table2, err := gpt.Read(disk2, int(diskfs.SectorSize512), int(diskfs.SectorSize512))
			Expect(err).ToNot(HaveOccurred())
			// check that its type GPT
			Expect(table2.Type()).To(Equal("gpt"))
			Expect(table2.Partitions).To(HaveLen(1))
			deviceLabel = table2.Partitions[0].Name
			Expect(deviceLabel).To(Equal("FAKELABEL"))
			Expect(table2.Partitions[0].Size).To(Equal(uint64(1024 * 1024 * 1024)))

		})
		It("Expands last partition after creating the partitions", func() {
			testConsole := console.New()
			testConsole.AddCmd(console.CmdMock{Cmd: "udevadm trigger && udevadm settle"})
			testConsole.AddCmd(console.CmdMock{Cmd: fmt.Sprintf("mkfs.ext2 %s1", devicePath)})
			testConsole.AddCmd(console.CmdMock{Cmd: "udevadm trigger && udevadm settle"})
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Path: devicePath},
					Parts:  []schema.Partition{{PLabel: label, Size: 512}},
					Expand: &schema.Expand{Size: 1024},
				},
			}, fs, testConsole)
			Expect(err).Should(BeNil())

			// Now check if the partition size is now 1024MiB
			disk, err := fileBackend.OpenFromPath(rawDevicePath, true)
			defer disk.Close()
			table, err := gpt.Read(disk, int(diskfs.SectorSize512), int(diskfs.SectorSize512))
			Expect(err).ToNot(HaveOccurred())
			// check that its type GPT
			Expect(table.Type()).To(Equal("gpt"))
			Expect(table.Partitions).To(HaveLen(1))
			deviceLabel = table.Partitions[0].Name
			Expect(deviceLabel).To(Equal("FAKELABEL"))
			Expect(table.Partitions[0].Size).To(Equal(uint64(1024 * 1024 * 1024)))

		})
		It("Expands last partition with XFS fs", func() {
			testConsole := console.New()
			testConsole.AddCmd(console.CmdMock{Cmd: "udevadm trigger && udevadm settle"})
			testConsole.AddCmd(console.CmdMock{Cmd: fmt.Sprintf("mkfs.xfs %s1", devicePath)})
			testConsole.AddCmd(console.CmdMock{Cmd: "udevadm trigger && udevadm settle"})
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Path: devicePath},
					Parts:  []schema.Partition{{PLabel: label, Size: 100, FileSystem: "xfs"}},
					Expand: &schema.Expand{Size: 1024},
				},
			}, fs, testConsole)
			Expect(err).Should(BeNil())

			// Now check if the partition size is now 1024MiB
			disk, err := fileBackend.OpenFromPath(rawDevicePath, true)
			defer disk.Close()
			table, err := gpt.Read(disk, int(diskfs.SectorSize512), int(diskfs.SectorSize512))
			Expect(err).ToNot(HaveOccurred())
			// check that its type GPT
			Expect(table.Type()).To(Equal("gpt"))
			Expect(table.Partitions).To(HaveLen(1))
			deviceLabel = table.Partitions[0].Name
			Expect(deviceLabel).To(Equal("FAKELABEL"))
			Expect(table.Partitions[0].Size).To(Equal(uint64(1024 * 1024 * 1024)))

		})
		It("Fails to expand last partition, if there is not enough space left", func() {
			testConsole := console.New()
			testConsole.AddCmd(console.CmdMock{Cmd: "udevadm trigger && udevadm settle"})
			testConsole.AddCmd(console.CmdMock{Cmd: fmt.Sprintf("mkfs.ext2 %s1", devicePath)})
			testConsole.AddCmd(console.CmdMock{Cmd: "udevadm trigger && udevadm settle"})
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Path: devicePath},
					Parts:  []schema.Partition{{PLabel: label, Size: 1000}},
				},
			}, fs, testConsole)
			Expect(err).Should(BeNil())
			// Now try to expand over its possible size
			err = Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Path: devicePath},
					Expand: &schema.Expand{Size: 3073},
				},
			}, fs, testConsole)
			Expect(err).To(HaveOccurred())
		})
		It("Fails on an xfs fs with a label longer than 12 chars", func() {
			testConsole := console.New()
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Path: devicePath},
					Parts:  []schema.Partition{{FSLabel: "LABEL_TOO_LONG_FOR_XFS", Size: 1024, FileSystem: "xfs"}},
				},
			}, fs, testConsole)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot be longer than 12 chars"))
		})
		It("Works on an non-xfs fs with a label longer than 12 chars", func() {
			label = "LABEL_TOO_LONG_FOR_XFS"
			testConsole := console.New()
			testConsole.AddCmd(console.CmdMock{Cmd: "udevadm trigger && udevadm settle"})
			testConsole.AddCmd(console.CmdMock{Cmd: fmt.Sprintf("mkfs.ext4 %s1", devicePath)})
			testConsole.AddCmd(console.CmdMock{Cmd: "udevadm trigger && udevadm settle"})
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Path: devicePath},
					Parts:  []schema.Partition{{PLabel: label, Size: 10, FileSystem: "ext4"}},
				},
			}, fs, testConsole)
			Expect(err).Should(BeNil())
		})
		It("Adds a swap partition and fails expanding it", func() {
			testConsole := console.New()
			testConsole.AddCmd(console.CmdMock{Cmd: "udevadm trigger && udevadm settle"})
			testConsole.AddCmd(console.CmdMock{Cmd: fmt.Sprintf("mkswap -L MYLABEL %s1", devicePath)})

			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Path: devicePath},
					Parts:  []schema.Partition{{FSLabel: "MYLABEL", Size: 10, FileSystem: "swap"}},
					Expand: &schema.Expand{Size: 500},
				},
			}, fs, testConsole)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("swap resizing is not supported"))
		})
	})
})

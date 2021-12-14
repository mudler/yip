package plugins_test

import (
	"fmt"

	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	console "github.com/mudler/yip/tests/console"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/vfst"
)

var deviceLabel = "reflabel"
var label = "MYLABEL"

var pTable console.CmdMock = console.CmdMock{
	Cmd: "sgdisk -p /some/device",
	Output: `Disk /some/device: 6471680 sectors, 3.1 GiB
Logical sector size: 512 bytes
Disk identifier (GUID): D2C09E82-250C-4A75-83B4-184BACC3D879
Partition table holds up to 128 entries
First usable sector is 34, last usable sector is 6471646
Partitions will be aligned on 2048-sector boundaries
Total free space is 4029 sectors (2.0 MiB)

Number  Start (sector)    End (sector)  Size       Code  Name
   1            2048            6143   2.0 MiB     EF02  legacy
   2            6144           47103   20.0 MiB    EF00  UEFI
   3           47104          178175   64.0 MiB    8300
   4          178176         4372479   2.0 GiB     8300  root`,
}

var lsblkTypes console.CmdMock = console.CmdMock{
	Cmd: "lsblk -ltnpo name,type /some/device",
	Output: `/some/device  disk
/some/device1 part
/some/device2 part
/some/device5 part`,
}

var sync console.CmdMock = console.CmdMock{
	Cmd: "sync",
}

var CmdsAddPartByDevPath []console.CmdMock = append([]console.CmdMock{
	{Cmd: "lsblk -npo type /some/device", Output: "loop"},
	{Cmd: "sgdisk --verify /some/device", Output: "the end of the disk"},
	{Cmd: "sgdisk -P -e /some/device"},
	{Cmd: "sgdisk -e /some/device"}, pTable,
	{Cmd: "udevadm settle"},
	{Cmd: "blkid -l --match-token LABEL=MYLABEL -o device"},
	{Cmd: "sgdisk -P -n=5:0:+2097152 -t=5:8300 /some/device"},
	{Cmd: "sgdisk -n=5:0:+2097152 -t=5:8300 /some/device"}, pTable,
	{Cmd: "udevadm settle"},
	{Cmd: "partprobe /some/device"}, sync,
	{Cmd: "udevadm settle"},
	lsblkTypes,
	{Cmd: "mkfs.ext2 -L MYLABEL /some/device5"},
	{Cmd: "udevadm settle"},
	{Cmd: "partprobe /some/device"},
	sync,
	{Cmd: "udevadm settle"},
	{Cmd: "blkid -l --match-token LABEL=MYLABEL -o device"},
})

var CmdsAddAlreadyExistingPart []console.CmdMock = []console.CmdMock{
	{Cmd: "udevadm settle"},
	{Cmd: "blkid -l --match-token LABEL=reflabel -o device", Output: "/some/part"},
	{Cmd: "lsblk -npo pkname /some/part", Output: "/some/device"},
	{Cmd: "sgdisk --verify /some/device", Output: "the end of the disk"},
	{Cmd: "sgdisk -P -e /some/device"},
	{Cmd: "sgdisk -e /some/device"}, pTable,
	{Cmd: "udevadm settle"},
	{Cmd: "blkid -l --match-token LABEL=MYLABEL -o device", Output: "/some/part"},
}

var CmdsExpandPart []console.CmdMock = []console.CmdMock{
	{Cmd: "udevadm settle"},
	{Cmd: "blkid -l --match-token LABEL=reflabel -o device", Output: "/some/part"},
	{Cmd: "lsblk -npo pkname /some/part", Output: "/some/device"},
	{Cmd: "sgdisk --verify /some/device", Output: "the end of the disk"},
	{Cmd: "sgdisk -P -e /some/device"},
	{Cmd: "sgdisk -e /some/device"}, pTable,
	{Cmd: "sgdisk -P -d=4 -n=4:178176:+6291456 -c=4:root -t=4:8300 /some/device"},
	{Cmd: "sgdisk -d=4 -n=4:178176:+6291456 -c=4:root -t=4:8300 /some/device"},
	{Cmd: "blkid /some/device4 -s TYPE -o value", Output: "ext4"},
	{Cmd: "e2fsck -fy /some/device4"},
	{Cmd: "resize2fs /some/device4"}, pTable,
	{Cmd: "udevadm settle"},
	{Cmd: "partprobe /some/device"},
	sync,
}

var CmdsExpandPartXfs []console.CmdMock = []console.CmdMock{
	{Cmd: "udevadm settle"},
	{Cmd: "blkid -l --match-token LABEL=reflabel -o device", Output: "/some/part"},
	{Cmd: "lsblk -npo pkname /some/part", Output: "/some/device"},
	{Cmd: "sgdisk --verify /some/device", Output: "the end of the disk"},
	{Cmd: "sgdisk -P -e /some/device"},
	{Cmd: "sgdisk -e /some/device"}, pTable,
	{Cmd: "sgdisk -P -d=4 -n=4:178176:+6291456 -c=4:root -t=4:8300 /some/device"},
	{Cmd: "sgdisk -d=4 -n=4:178176:+6291456 -c=4:root -t=4:8300 /some/device"},
	{Cmd: "blkid /some/device4 -s TYPE -o value", Output: "xfs"},
	{Cmd: "mount -t xfs /some/device4 /tmp/*", UseRegexp: true},
	{Cmd: "xfs_growfs /tmp/*", UseRegexp: true},
	{Cmd: "umount /tmp/*", UseRegexp: true},
	pTable,
	{Cmd: "udevadm settle"},
	{Cmd: "partprobe /some/device"},
	sync,
}

func CmdsAddPartByLabel(fs string) []console.CmdMock {
	return []console.CmdMock{
		{Cmd: "udevadm settle"},
		{Cmd: fmt.Sprintf("blkid -l --match-token LABEL=%s -o device", deviceLabel), Output: "/some/part"},
		{Cmd: "lsblk -npo pkname /some/part", Output: "/some/device"},
		{Cmd: "sgdisk --verify /some/device", Output: "the end of the disk"},
		{Cmd: "sgdisk -P -e /some/device"},
		{Cmd: "sgdisk -e /some/device"}, pTable,
		{Cmd: "udevadm settle"},
		{Cmd: fmt.Sprintf("blkid -l --match-token LABEL=%s -o device", label)},
		{Cmd: "sgdisk -P -n=5:0:+2097152 -t=5:8300 /some/device"},
		{Cmd: "sgdisk -n=5:0:+2097152 -t=5:8300 /some/device"}, pTable,
		{Cmd: "udevadm settle"},
		{Cmd: "partprobe /some/device"}, sync,
		{Cmd: "udevadm settle"},
		lsblkTypes,
		{Cmd: fmt.Sprintf("mkfs.%s -L %s /some/device5", fs, label)},
		{Cmd: "udevadm settle"},
		{Cmd: "partprobe /some/device"},
		sync,
		{Cmd: "udevadm settle"},
		{Cmd: fmt.Sprintf("blkid -l --match-token LABEL=%s -o device", label)},
	}
}

var _ = Describe("Layout", func() {
	Context("creating", func() {
		fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{})
		Expect(err).Should(BeNil())
		defer cleanup()

		l := logrus.New()

		It("Adds a new partition of 1024MiB in reflabel device", func() {
			testConsole := console.New()
			testConsole.AddCmds(CmdsAddPartByLabel("ext2"))
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Label: deviceLabel},
					Parts:  []schema.Partition{{FSLabel: label, Size: 1024}},
				},
			}, fs, testConsole)
			Expect(err).Should(BeNil())
		})
		It("Adds a new partition of 1024MiB in /some/device device", func() {
			testConsole := console.New()
			testConsole.AddCmds(CmdsAddPartByDevPath)
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Path: "/some/device"},
					Parts:  []schema.Partition{{FSLabel: "MYLABEL", Size: 1024}},
				},
			}, fs, testConsole)
			Expect(err).Should(BeNil())
		})
		It("Fails to add a partition of 1030MiB, there are only 1024MiB available", func() {
			testConsole := console.New()
			testConsole.AddCmds(CmdsAddPartByLabel("ext2"))
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Label: deviceLabel},
					Parts:  []schema.Partition{{FSLabel: label, Size: 1025}},
				},
			}, fs, testConsole)
			Expect(err).To(HaveOccurred())
		})
		It("Ignores an already existing partition", func() {
			testConsole := console.New()
			testConsole.AddCmds(CmdsAddAlreadyExistingPart)
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Label: deviceLabel},
					Parts:  []schema.Partition{{FSLabel: label, Size: 1024}},
				},
			}, fs, testConsole)
			Expect(err).Should(BeNil())
		})
		It("Fails to expand last partition, it can't shrink a partition", func() {
			testConsole := console.New()
			testConsole.AddCmds(CmdsExpandPart)
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Label: deviceLabel},
					Expand: &schema.Expand{Size: 1024},
				},
			}, fs, testConsole)
			Expect(err).To(HaveOccurred())
		})
		It("Expands last partition", func() {
			testConsole := console.New()
			testConsole.AddCmds(CmdsExpandPart)
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Label: deviceLabel},
					Expand: &schema.Expand{Size: 3072},
				},
			}, fs, testConsole)
			Expect(err).Should(BeNil())
		})
		It("Expands last partition with XFS fs", func() {
			testConsole := console.New()
			testConsole.AddCmds(CmdsExpandPartXfs)
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Label: "reflabel"},
					Expand: &schema.Expand{Size: 3072},
				},
			}, fs, testConsole)
			Expect(err).Should(BeNil())
		})
		It("Fails to expand last partition, max size is 3072MiB", func() {
			testConsole := console.New()
			testConsole.AddCmds(CmdsExpandPart)
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Label: deviceLabel},
					Expand: &schema.Expand{Size: 3073},
				},
			}, fs, testConsole)
			Expect(err).To(HaveOccurred())
		})
		It("Fails on an xfs fs with a label longer than 12 chars", func() {
			testConsole := console.New()
			err := Layout(l, schema.Stage{
				Layout: schema.Layout{
					Device: &schema.Device{Label: deviceLabel},
					Parts:  []schema.Partition{{FSLabel: "LABEL_TOO_LONG_FOR_XFS", Size: 1024, FileSystem: "xfs"}},
				},
			}, fs, testConsole)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot have a label longer than 12 chars"))
		})

		It("Works on an non-xfs fs with a label longer than 12 chars", func() {
			label = "LABEL_TOO_LONG_FOR_XFS"
			for _, filesystem := range []string{"ext2", "ext3", "ext4"} {
				testConsole := console.New()
				testConsole.AddCmds(CmdsAddPartByLabel(filesystem))
				err := Layout(l, schema.Stage{
					Layout: schema.Layout{
						Device: &schema.Device{Label: deviceLabel},
						Parts:  []schema.Partition{{FSLabel: label, Size: 1024, FileSystem: filesystem}},
					},
				}, fs, testConsole)
				Expect(err).ToNot(HaveOccurred())
			}
		})
	})
})

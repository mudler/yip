package plugins_test

import (
	"debug/elf"
	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v4"
	"os"
)

var _ = Describe("UnpackImage", Label("unpack_image"), func() {
	var testConsole consoletests.TestConsole
	var fs vfs.FS
	var err error
	var target string
	BeforeEach(func() {
		// Only run this tests if we are root
		if os.Geteuid() != 0 {
			Skip("Skipping tests, must be run as root for extraction to work")
		}
		target = "/tmp/unpack/"
		// Check that dir is not there
		_, err = os.Stat(target)
		Expect(err).Should(HaveOccurred())
		testConsole = consoletests.TestConsole{}
		fs = vfs.OSFS
	})
	AfterEach(func() {
		consoletests.Reset()
		Expect(os.RemoveAll(target)).ToNot(HaveOccurred())
	})

	Describe("UnpackImage", func() {
		It("Extracts", func() {
			err = UnpackImage(logrus.New(), schema.Stage{
				UnpackImages: []schema.UnpackImageConf{
					{
						Source: "quay.io/luet/base:latest",
						Target: target,
					},
				},
			}, fs, testConsole)

			Expect(err).ShouldNot(HaveOccurred())
			_, err := os.Stat(target)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = os.Stat("/tmp/unpack/usr/bin/luet")
			Expect(err).ShouldNot(HaveOccurred())
		})
		It("Extracts for a different platform", func() {
			err = UnpackImage(logrus.New(), schema.Stage{
				UnpackImages: []schema.UnpackImageConf{
					{
						Source:   "quay.io/luet/base:latest",
						Target:   target,
						Platform: "linux/arm64",
					},
				},
			}, fs, testConsole)

			Expect(err).ShouldNot(HaveOccurred())
			_, err := os.Stat(target)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = os.Stat("/tmp/unpack/usr/bin/luet")
			Expect(err).ShouldNot(HaveOccurred())
			// Check if binary is arm64
			isARM, err := isARMBinary("/tmp/unpack/usr/bin/luet")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(isARM).Should(BeTrue())
		})
	})
})

// isARMBinary checks if a binary is ARM or ARM64
func isARMBinary(filePath string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	elfFile, err := elf.NewFile(file)
	if err != nil {
		return false, err
	}

	switch elfFile.Machine {
	case elf.EM_ARM, elf.EM_AARCH64:
		return true, nil
	default:
		return false, nil
	}
}

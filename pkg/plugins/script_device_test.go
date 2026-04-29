package plugins_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/mudler/yip/pkg/plugins"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResolveScriptDevice", Label("script-device"), func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "resolve-script-device-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	It("returns a plain path unchanged", func() {
		result, err := ResolveScriptDevice("/dev/sda")
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal("/dev/sda"))
	})

	It("executes the script and returns the trimmed stdout as the device path", func() {
		script := filepath.Join(tmpDir, "pick-disk.sh")
		Expect(os.WriteFile(script, []byte("#!/bin/sh\necho /dev/sda\n"), 0755)).To(Succeed())

		result, err := ResolveScriptDevice(fmt.Sprintf("script://%s", script))
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal("/dev/sda"))
	})

	It("trims leading and trailing whitespace from stdout", func() {
		script := filepath.Join(tmpDir, "pick-disk.sh")
		Expect(os.WriteFile(script, []byte("#!/bin/sh\nprintf '  /dev/vda  '\n"), 0755)).To(Succeed())

		result, err := ResolveScriptDevice(fmt.Sprintf("script://%s", script))
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal("/dev/vda"))
	})

	It("returns an error when the script exits with a non-zero code", func() {
		script := filepath.Join(tmpDir, "fail.sh")
		Expect(os.WriteFile(script, []byte("#!/bin/sh\necho 'something went wrong' >&2\nexit 1\n"), 0755)).To(Succeed())

		_, err := ResolveScriptDevice(fmt.Sprintf("script://%s", script))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("something went wrong"))
	})

	It("returns an error when the script produces no output", func() {
		script := filepath.Join(tmpDir, "empty.sh")
		Expect(os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0755)).To(Succeed())

		_, err := ResolveScriptDevice(fmt.Sprintf("script://%s", script))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("empty"))
	})

	It("returns an error when the script path does not exist", func() {
		_, err := ResolveScriptDevice("script:///nonexistent/pick-disk.sh")
		Expect(err).To(HaveOccurred())
	})

	It("passes arguments to the script", func() {
		script := filepath.Join(tmpDir, "with-args.sh")
		Expect(os.WriteFile(script, []byte("#!/bin/sh\necho $1\n"), 0755)).To(Succeed())

		result, err := ResolveScriptDevice(fmt.Sprintf("script://%s /dev/nvme0n1", script))
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal("/dev/nvme0n1"))
	})
})

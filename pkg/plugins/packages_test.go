package plugins

import (
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sanity-io/litter"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v4/vfst"
	"io"
)

var _ = Describe("Commands", func() {
	Context("parsing yip file", func() {
		testConsole := consoletests.TestConsole{}
		l := logrus.New()
		l.SetOutput(io.Discard)

		BeforeEach(func() {
			consoletests.Reset()
		})
		It("execute proper install commands", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			defer cleanup()
			Expect(fs.Mkdir("/etc", 0755)).ToNot(HaveOccurred())
			Expect(fs.WriteFile("/etc/os-release", []byte("ID=debian\nVERSION=10\n"), 0644)).ToNot(HaveOccurred())

			err = Packages(l, schema.Stage{
				Packages: schema.Packages{
					Install: []string{"foo", "bar"},
					Remove:  []string{"baz", "qux"},
					Refresh: true,
				},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(consoletests.Commands).Should(Equal([]string{"apt-get -y update", "apt-get -y --no-install-recommends install foo bar", "apt-get -y remove baz qux"}))
		})
		It("execute proper install commands for different OS", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			defer cleanup()
			Expect(fs.Mkdir("/etc", 0755)).ToNot(HaveOccurred())
			stage := schema.Stage{
				Packages: schema.Packages{
					Install: []string{"foo", "bar"},
					Remove:  []string{"baz", "qux"},
					Refresh: true,
				},
			}
			type test struct {
				osRelease string
				expected  []string
			}
			tests := []test{
				{
					osRelease: "ID=debian\nVERSION=10\n",
					expected:  []string{"apt-get -y update", "apt-get -y --no-install-recommends install foo bar", "apt-get -y remove baz qux"},
				},
				{
					osRelease: "ID=debian\nVERSION=11\n",
					expected:  []string{"apt-get -y update", "apt-get -y --no-install-recommends install foo bar", "apt-get -y remove baz qux"},
				},
				{
					osRelease: "ID=ubuntu\nVERSION=20.04\n",
					expected:  []string{"apt-get -y update", "apt-get -y --no-install-recommends install foo bar", "apt-get -y remove baz qux"},
				},
				{
					osRelease: "ID=centos\nVERSION=8\n",
					expected:  []string{"dnf -y update", "dnf -y install foo bar", "dnf -y remove baz qux"},
				},
				{
					osRelease: "ID=fedora\nVERSION=34\n",
					expected:  []string{"dnf -y update", "dnf -y install foo bar", "dnf -y remove baz qux"},
				},
				{
					osRelease: "ID=alpine\nVERSION=3.14\n",
					expected:  []string{"apk update", "apk add --no-cache foo bar", "apk del --no-cache baz qux"},
				},
				{
					osRelease: "ID=opensuse-leap\nVERSION=15.3\n",
					expected:  []string{"zypper -y update", "zypper -y install foo bar", "zypper -y remove baz qux"},
				},
				{
					osRelease: "ID=arch\nVERSION=rolling\n",
					expected:  []string{"pacman -Sy", "pacman -S --noconfirm foo bar", "pacman -R --noconfirm baz qux"},
				},
			}

			for _, t := range tests {
				Expect(fs.WriteFile("/etc/os-release", []byte(t.osRelease), 0644)).ToNot(HaveOccurred())
				err = Packages(l, stage, fs, testConsole)
				Expect(err).ShouldNot(HaveOccurred(), t.osRelease)
				Expect(consoletests.Commands).Should(Equal(t.expected), litter.Sdump(t.osRelease))
				consoletests.Reset()
			}
		})
		It("fails if it cant identify the systems package manager", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			defer cleanup()
			err = Packages(l, schema.Stage{
				Packages: schema.Packages{
					Install: []string{"foo", "bar"},
					Remove:  []string{"baz", "qux"},
					Refresh: true,
				},
			}, fs, testConsole)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown package manager"))
		})
	})
})

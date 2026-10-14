//   Copyright 2021 Ettore Di Giacinto <mudler@mocaccino.org>
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package plugins_test

import (
	"os"

	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v5"
	"github.com/twpayne/go-vfs/v5/vfst"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Dns", func() {
	Context("parsing yip file", func() {
		testConsole := consoletests.TestConsole{}

		var fs vfs.FS
		var cleanup func()
		var err error

		BeforeEach(func() {
			testConsole.Reset()
		})

		AfterEach(func() {
			if cleanup != nil {
				cleanup()
			}
		})

		It("sets dns", func() {
			fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{"/tmp/test/bar": "boo"})
			Expect(err).Should(BeNil())

			err = DNS(logrus.New(), schema.Stage{Dns: schema.DNS{Path: "/tmp/test/foo", Nameservers: []string{"8.8.8.8"}}}, fs, &testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			b, err := fs.ReadFile("/tmp/test/foo")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(b)).Should(Equal("nameserver 8.8.8.8\n"))
		})

		It("writes search and options too", func() {
			fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{"/tmp/test/bar": "boo"})
			Expect(err).Should(BeNil())

			err = DNS(logrus.New(), schema.Stage{Dns: schema.DNS{
				Path:        "/tmp/test/foo",
				Nameservers: []string{"8.8.8.8", "8.8.4.4"},
				DnsSearch:   []string{"kairos.io"},
				DnsOptions:  []string{"ndots:1"},
			}}, fs, &testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			b, err := fs.ReadFile("/tmp/test/foo")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(b)).Should(Equal("search kairos.io\nnameserver 8.8.8.8\nnameserver 8.8.4.4\noptions ndots:1\n"))
		})

		// The plugin is handed a vfs.FS and used to ignore it, writing the host's
		// real /etc/resolv.conf instead of the one inside the filesystem it was
		// given.
		It("writes inside the filesystem it is given", func() {
			fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{"/etc/resolv.conf": "nameserver 127.0.0.53\n"})
			Expect(err).Should(BeNil())

			err = DNS(logrus.New(), schema.Stage{Dns: schema.DNS{Nameservers: []string{"8.8.8.8"}}}, fs, &testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			b, err := fs.ReadFile("/etc/resolv.conf")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(b)).Should(Equal("nameserver 8.8.8.8\n"))

			// The host file must not have been touched.
			host, err := os.ReadFile("/etc/resolv.conf")
			if err == nil {
				Expect(string(host)).ShouldNot(Equal("nameserver 8.8.8.8\n"))
			}
		})

		// Kairos points /etc/resolv.conf at /run/systemd/resolve/resolv.conf, so
		// following the symlink writes a file systemd-resolved owns and
		// regenerates, and the nameservers never stick.
		It("replaces a symlink instead of writing through it", func() {
			fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{
				"/run/systemd/resolve/resolv.conf": "nameserver 127.0.0.53\n",
			})
			Expect(err).Should(BeNil())
			Expect(fs.Mkdir("/etc", 0o755)).Should(Succeed())
			Expect(fs.Symlink("/run/systemd/resolve/resolv.conf", "/etc/resolv.conf")).Should(Succeed())

			err = DNS(logrus.New(), schema.Stage{Dns: schema.DNS{Nameservers: []string{"8.8.8.8"}}}, fs, &testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			info, err := fs.Lstat("/etc/resolv.conf")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(info.Mode() & os.ModeSymlink).Should(Equal(os.FileMode(0)))

			b, err := fs.ReadFile("/etc/resolv.conf")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(b)).Should(Equal("nameserver 8.8.8.8\n"))

			// systemd-resolved's own file is left alone.
			b, err = fs.ReadFile("/run/systemd/resolve/resolv.conf")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(b)).Should(Equal("nameserver 127.0.0.53\n"))
		})

		// In the initramfs /run/systemd/resolve does not exist yet, so following
		// the dangling symlink failed the whole stage with ENOENT.
		It("replaces a dangling symlink", func() {
			fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{"/tmp/test/bar": "boo"})
			Expect(err).Should(BeNil())
			Expect(fs.Mkdir("/etc", 0o755)).Should(Succeed())
			Expect(fs.Symlink("/run/systemd/resolve/resolv.conf", "/etc/resolv.conf")).Should(Succeed())

			err = DNS(logrus.New(), schema.Stage{Dns: schema.DNS{Nameservers: []string{"8.8.8.8"}}}, fs, &testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			b, err := fs.ReadFile("/etc/resolv.conf")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(b)).Should(Equal("nameserver 8.8.8.8\n"))
		})

		It("creates the parent directory when it is missing", func() {
			fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{"/tmp/test/bar": "boo"})
			Expect(err).Should(BeNil())

			err = DNS(logrus.New(), schema.Stage{Dns: schema.DNS{Path: "/etc/resolv.conf", Nameservers: []string{"8.8.8.8"}}}, fs, &testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			b, err := fs.ReadFile("/etc/resolv.conf")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(b)).Should(Equal("nameserver 8.8.8.8\n"))
		})

		It("does nothing without nameservers", func() {
			fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{"/etc/resolv.conf": "nameserver 127.0.0.53\n"})
			Expect(err).Should(BeNil())

			err = DNS(logrus.New(), schema.Stage{}, fs, &testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			b, err := fs.ReadFile("/etc/resolv.conf")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(b)).Should(Equal("nameserver 127.0.0.53\n"))
		})
	})
})

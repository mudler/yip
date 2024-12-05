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
	"bytes"
	"fmt"
	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v4/vfst"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Systemctl", func() {
	Context("parsing yip file", func() {
		testConsole := consoletests.TestConsole{}
		BeforeEach(func() {
			consoletests.Reset()
		})
		It("starts and enables services", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = Systemctl(logrus.New(), schema.Stage{
				Systemctl: schema.Systemctl{
					Enable:  []string{"foo"},
					Disable: []string{"bar"},
					Mask:    []string{"baz"},
					Start:   []string{"moz"},
				},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(consoletests.Commands).Should(Equal([]string{"systemctl enable foo", "systemctl disable bar", "systemctl mask baz", "systemctl start moz"}))
		})
		Context("Overrides", func() {
			It("creates override files", func() {
				fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{})
				Expect(err).Should(BeNil())
				defer cleanup()

				err = Systemctl(logrus.New(), schema.Stage{
					Systemctl: schema.Systemctl{
						Overrides: []schema.SystemctlOverride{
							{
								Service: "foo.service",
								Content: "[Unit]\nbar=baz",
							},
						},
					},
				}, fs, testConsole)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(fs.Stat("/etc/systemd/system/foo.service.d/override-yip.conf")).ToNot(BeNil())
				Expect(consoletests.Commands).Should(BeEmpty())
				content, err := fs.ReadFile("/etc/systemd/system/foo.service.d/override-yip.conf")
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).Should(Equal("[Unit]\nbar=baz"))
			})
			It("creates override files if service is given without extension", func() {
				fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{})
				Expect(err).Should(BeNil())
				defer cleanup()

				err = Systemctl(logrus.New(), schema.Stage{
					Systemctl: schema.Systemctl{
						Overrides: []schema.SystemctlOverride{
							{
								Service: "foo",
								Content: "[Unit]\nbar=baz",
							},
						},
					},
				}, fs, testConsole)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(fs.Stat("/etc/systemd/system/foo.service.d/override-yip.conf")).ToNot(BeNil())
				Expect(consoletests.Commands).Should(BeEmpty())
				content, err := fs.ReadFile("/etc/systemd/system/foo.service.d/override-yip.conf")
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).Should(Equal("[Unit]\nbar=baz"))
			})
			It("creates override files with custom override file name", func() {
				fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{})
				Expect(err).Should(BeNil())
				defer cleanup()

				err = Systemctl(logrus.New(), schema.Stage{
					Systemctl: schema.Systemctl{
						Overrides: []schema.SystemctlOverride{
							{
								Service: "foo.service",
								Content: "[Unit]\nbar=baz",
								Name:    "override-foo.conf",
							},
						},
					},
				}, fs, testConsole)
				Expect(err).ShouldNot(HaveOccurred())
				_, err = fs.Stat("/etc/systemd/system/foo.service.d/override-yip.conf")
				Expect(err).ToNot(BeNil())
				_, err = fs.Stat("/etc/systemd/system/foo.service.d/override-foo.conf")
				Expect(err).To(BeNil())
				Expect(consoletests.Commands).Should(BeEmpty())
				content, err := fs.ReadFile("/etc/systemd/system/foo.service.d/override-foo.conf")
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).Should(Equal("[Unit]\nbar=baz"))
			})
			It("creates override files with custom override file name missing the extension", func() {
				fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{})
				Expect(err).Should(BeNil())
				defer cleanup()

				err = Systemctl(logrus.New(), schema.Stage{
					Systemctl: schema.Systemctl{
						Overrides: []schema.SystemctlOverride{
							{
								Service: "foo.service",
								Content: "[Unit]\nbar=baz",
								Name:    "override-foo",
							},
						},
					},
				}, fs, testConsole)
				Expect(err).ShouldNot(HaveOccurred())
				_, err = fs.Stat("/etc/systemd/system/foo.service.d/override-yip.conf")
				Expect(err).ToNot(BeNil())
				_, err = fs.Stat("/etc/systemd/system/foo.service.d/override-foo.conf")
				Expect(err).To(BeNil())
				Expect(consoletests.Commands).Should(BeEmpty())
				content, err := fs.ReadFile("/etc/systemd/system/foo.service.d/override-foo.conf")
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).Should(Equal("[Unit]\nbar=baz"))
			})
			It("doesn't do anything if service name is missing", func() {
				fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{})
				Expect(err).Should(BeNil())
				defer cleanup()
				var buf bytes.Buffer
				l := logrus.New()
				l.SetOutput(&buf)
				err = Systemctl(l, schema.Stage{
					Systemctl: schema.Systemctl{
						Overrides: []schema.SystemctlOverride{
							{
								Service: "",
								Content: "[Unit]\nbar=baz",
							},
						},
					},
				}, fs, testConsole)
				Expect(err).ToNot(HaveOccurred())
				// Should not create the directory
				_, err = fs.Stat("/etc/systemd/system/")
				Expect(err).To(HaveOccurred())
				Expect(consoletests.Commands).Should(BeEmpty())
				Expect(buf.String()).Should(ContainSubstring(ErrorEmptyOverrideService))
			})
			It("doesn't do anything if content is missing", func() {
				fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{})
				Expect(err).Should(BeNil())
				defer cleanup()
				var buf bytes.Buffer
				l := logrus.New()
				l.SetOutput(&buf)
				err = Systemctl(l, schema.Stage{
					Systemctl: schema.Systemctl{
						Overrides: []schema.SystemctlOverride{
							{
								Service: "test.service",
								Content: "",
							},
						},
					},
				}, fs, testConsole)
				Expect(err).ToNot(HaveOccurred())
				// Should not create the directory
				_, err = fs.Stat("/etc/systemd/system/")
				Expect(err).To(HaveOccurred())
				Expect(consoletests.Commands).Should(BeEmpty())
				Expect(buf.String()).Should(ContainSubstring(fmt.Sprintf(ErrorEmptyOverrideContent, "test.service")))
			})
		})
	})
})

// Copyright © 2020 Ettore Di Giacinto <mudler@gentoo.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package schema_test

import (
	. "github.com/mudler/yip/pkg/schema"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/twpayne/go-vfs/vfst"
)

func loadstdYip(s string) *YipConfig {
	fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/yip.yaml": s, "/etc/passwd": ""})
	Expect(err).Should(BeNil())
	defer cleanup()

	yipConfig, err := Load("/yip.yaml", fs, FromFile, nil)
	Expect(err).ToNot(HaveOccurred())
	return yipConfig
}

func loadYip(s string) *YipConfig {
	fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/yip.yaml": s})
	Expect(err).Should(BeNil())
	defer cleanup()

	yipConfig, err := Load("/yip.yaml", fs, FromFile, DotNotationModifier)
	Expect(err).ToNot(HaveOccurred())
	return yipConfig
}

var _ = Describe("Schema", func() {
	Context("Loading from dot notation", func() {
		oneConfigwithGarbageS := "stages.foo[0].name=bar boo.baz"
		twoConfigsS := "stages.foo[0].name=bar   stages.foo[0].commands[0]=baz"
		threeConfigInvalid := `ip=dhcp test="echo ping_test_host=127.0.0.1  > /tmp/jojo"`
		fourConfigHalfInvalid := `stages.foo[0].name=bar ip=dhcp test="echo ping_test_host=127.0.0.1  > /tmp/dio"`

		It("Reads yip file correctly", func() {
			yipConfig := loadYip(oneConfigwithGarbageS)
			Expect(yipConfig.Stages["foo"][0].Name).To(Equal("bar"))
		})
		It("Reads yip file correctly", func() {
			yipConfig := loadYip(twoConfigsS)
			Expect(yipConfig.Stages["foo"][0].Name).To(Equal("bar"))
			Expect(yipConfig.Stages["foo"][0].Commands[0]).To(Equal("baz"))
		})

		It("Reads yip file correctly", func() {
			yipConfig, err := Load(twoConfigsS, nil, nil, DotNotationModifier)
			Expect(err).ToNot(HaveOccurred())
			Expect(yipConfig.Stages["foo"][0].Name).To(Equal("bar"))
			Expect(yipConfig.Stages["foo"][0].Commands[0]).To(Equal("baz"))
		})

		It("Reads yip file correctly", func() {
			yipConfig, err := Load(threeConfigInvalid, nil, nil, DotNotationModifier)
			Expect(err).ToNot(HaveOccurred())
			// should look like an empty yipConfig as its an invalid config, so nothing should be loaded
			Expect(yipConfig.Stages).To(Equal(YipConfig{}.Stages))
			Expect(yipConfig.Name).To(Equal(YipConfig{}.Name))
		})

		It("Reads yip file correctly", func() {
			yipConfig, err := Load(fourConfigHalfInvalid, nil, nil, DotNotationModifier)
			Expect(err).ToNot(HaveOccurred())
			Expect(yipConfig.Name).To(Equal(YipConfig{}.Name))
			// Even if broken config, it should load the valid parts of the config
			Expect(yipConfig.Stages["foo"][0].Name).To(Equal("bar"))
		})
	})

	Context("Loading CloudConfig", func() {
		It("Reads cloudconfig to boot stage", func() {
			yipConfig := loadstdYip(`#cloud-config
growpart:
 devices: ['/']
stages:
  test:
  - environment:
      foo: bar
users:
- name: "bar"
  passwd: "foo"
  uid: "1002"
  lock_passwd: true
  groups: "users"
  ssh_authorized_keys:
  - faaapploo
ssh_authorized_keys:
  - asdd
runcmd:
- foo
hostname: "bar"
write_files:
- encoding: b64
  content: CiMgVGhpcyBmaWxlIGNvbnRyb2xzIHRoZSBzdGF0ZSBvZiBTRUxpbnV4
  path: /foo/bar
  permissions: "0644"
  owner: "bar"
`)
			Expect(len(yipConfig.Stages)).To(Equal(3))
			Expect(yipConfig.Stages["boot"][0].Users["bar"].UID).To(Equal("1002"))
			Expect(yipConfig.Stages["boot"][0].Users["bar"].PasswordHash).To(Equal("foo"))
			Expect(yipConfig.Stages["boot"][0].SSHKeys).To(Equal(map[string][]string{"bar": {"faaapploo", "asdd"}}))
			Expect(yipConfig.Stages["boot"][0].Files[0].Path).To(Equal("/foo/bar"))
			Expect(yipConfig.Stages["boot"][0].Files[0].Permissions).To(Equal(uint32(0644)))
			Expect(yipConfig.Stages["boot"][0].Hostname).To(Equal(""))
			Expect(yipConfig.Stages["initramfs"][0].Hostname).To(Equal("bar"))
			Expect(yipConfig.Stages["boot"][0].Commands).To(Equal([]string{"foo"}))
			Expect(yipConfig.Stages["test"][0].Environment["foo"]).To(Equal("bar"))
			Expect(yipConfig.Stages["boot"][0].Users["bar"].LockPasswd).To(Equal(true))
			Expect(yipConfig.Stages["boot"][1].Layout.Expand.Size).To(Equal(uint(0)))
			Expect(yipConfig.Stages["boot"][1].Layout.Device.Path).To(Equal("/"))
		})
	})
})

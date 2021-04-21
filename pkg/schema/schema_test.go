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
	"github.com/twpayne/go-vfs/vfst"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
	})

	Context("Loading CloudConfig", func() {
		It("Reads cloudconfig to boot stage", func() {
			yipConfig := loadstdYip(`#cloud-config
users:
- name: "bar"
  passwd: "foo"
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
			Expect(len(yipConfig.Stages)).To(Equal(1))
			Expect(yipConfig.Stages["boot"][0].Users["bar"].PasswordHash).To(Equal("foo"))
			Expect(yipConfig.Stages["boot"][0].SSHKeys).To(Equal(map[string][]string{"bar": {"faaapploo", "asdd"}}))
			Expect(yipConfig.Stages["boot"][0].Files[0].Path).To(Equal("/foo/bar"))
			Expect(yipConfig.Stages["boot"][0].Hostname).To(Equal("bar"))
			Expect(yipConfig.Stages["boot"][0].Commands).To(Equal([]string{"foo"}))

		})
	})
})

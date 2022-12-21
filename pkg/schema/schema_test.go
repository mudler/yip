//   Copyright 2020 Ettore Di Giacinto <mudler@mocaccino.org>
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
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
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
	Context("NetworkManager schema", func() {
		It("Loads it correctly", func() {
			yipConfig := loadstdYip(`
stages:
  default:
    - networkmanager:
      - name: "Connection1"
        connection:
          - interface-name: "wlan0"
          - type: "wifi"
        wifi:
          - ssid: "testSSID"
          - mode: "infrastructure"
        wifisecurity:
          - key-mgmt: "wpa-psk"
          - psk: "123456789"
      - name: "Connection2"
        connection:
          - interface-name: "wlan1"
          - type: "wifi"
        wifi:
          - ssid: "testSSID"
          - mode: "infrastructure"
        wifisecurity:
          - key-mgmt: "wpa-psk"
          - psk: "123456789"
        x8021:
          - key: "value"
        olpcmesh:
          - key: "value"
`)
			// Should have 2 connections
			Expect(len(yipConfig.Stages["default"][0].NetworkManager)).To(Equal(2))
			// Check values
			Expect(yipConfig.Stages["default"][0].NetworkManager[0].Name).To(Equal("Connection1"))
			Expect(yipConfig.Stages["default"][0].NetworkManager[1].Name).To(Equal("Connection2"))
			Expect(len(yipConfig.Stages["default"][0].NetworkManager[0].Connection)).To(Equal(2))
			Expect(len(yipConfig.Stages["default"][0].NetworkManager[1].Connection)).To(Equal(2))
			Expect(yipConfig.Stages["default"][0].NetworkManager[0].Connection).To(ContainElement(map[string]string{"interface-name": "wlan0"}))
			Expect(yipConfig.Stages["default"][0].NetworkManager[1].Connection).To(ContainElement(map[string]string{"interface-name": "wlan1"}))
			Expect(len(yipConfig.Stages["default"][0].NetworkManager[0].Wifi)).To(Equal(2))
			Expect(len(yipConfig.Stages["default"][0].NetworkManager[1].Wifi)).To(Equal(2))
			Expect(yipConfig.Stages["default"][0].NetworkManager[0].Wifi).To(ContainElement(map[string]string{"ssid": "testSSID"}))
			Expect(yipConfig.Stages["default"][0].NetworkManager[1].Wifi).To(ContainElement(map[string]string{"ssid": "testSSID"}))
			Expect(yipConfig.Stages["default"][0].NetworkManager[1].X8021).To(ContainElement(map[string]string{"key": "value"}))
			Expect(yipConfig.Stages["default"][0].NetworkManager[1].Olpcmesh).To(ContainElement(map[string]string{"key": "value"}))

		})
	})
})

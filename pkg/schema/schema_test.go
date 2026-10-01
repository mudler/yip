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
	"syscall"
	"time"

	. "github.com/mudler/yip/pkg/schema"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/twpayne/go-vfs/v5"
	"github.com/twpayne/go-vfs/v5/vfst"
)

const fileContent = `
Line1
Line2
Line3`

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
	It("loads subordinate uid and gid ranges", func() {
		yipConfig := loadstdYip(`stages:
  boot:
    - users:
        podman:
          subuid: "100000:65536"
          subgid: "200000:65536"
`)

		user := yipConfig.Stages["boot"][0].Users["podman"]
		Expect(user.SubUID).To(Equal("100000:65536"))
		Expect(user.SubGID).To(Equal("200000:65536"))
	})

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
  groups:
  - sudo
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
			Expect(len(yipConfig.Stages)).To(Equal(4))
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
			Expect(yipConfig.Stages["boot"][1].Layout.Expand.Size).To(Equal(uint64(0)))
			Expect(yipConfig.Stages["boot"][1].Layout.Device.Path).To(Equal("/"))
		})
		It("Reads sshkeys to network stage if they require network", func() {
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
  groups:
  - sudo
  ssh_authorized_keys:
  - gitlab:test
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
			Expect(len(yipConfig.Stages)).To(Equal(4))
			Expect(yipConfig.Stages["boot"][0].Users["bar"].UID).To(Equal("1002"))
			Expect(yipConfig.Stages["boot"][0].Users["bar"].PasswordHash).To(Equal("foo"))
			Expect(len(yipConfig.Stages["boot"][0].SSHKeys)).To(Equal(1))
			Expect(yipConfig.Stages["boot"][0].SSHKeys).To(Equal(map[string][]string{"bar": {"asdd"}}))
			Expect(yipConfig.Stages["boot"][0].Files[0].Path).To(Equal("/foo/bar"))
			Expect(yipConfig.Stages["boot"][0].Files[0].Permissions).To(Equal(uint32(0644)))
			Expect(yipConfig.Stages["boot"][0].Hostname).To(Equal(""))
			Expect(yipConfig.Stages["initramfs"][0].Hostname).To(Equal("bar"))
			Expect(yipConfig.Stages["boot"][0].Commands).To(Equal([]string{"foo"}))
			Expect(yipConfig.Stages["test"][0].Environment["foo"]).To(Equal("bar"))
			Expect(yipConfig.Stages["boot"][0].Users["bar"].LockPasswd).To(Equal(true))
			Expect(yipConfig.Stages["boot"][1].Layout.Expand.Size).To(Equal(uint64(0)))
			Expect(yipConfig.Stages["boot"][1].Layout.Device.Path).To(Equal("/"))
			// if just one key needs network, it should go to the network stage
			Expect(len(yipConfig.Stages["network"][0].SSHKeys)).To(Equal(1))
			Expect(yipConfig.Stages["network"][0].SSHKeys).To(Equal(map[string][]string{"bar": {"gitlab:test"}}))
		})

		It("Reads cloudconfig with a jinja header", func() {
			yipConfig := loadstdYip(`## template: jinja
#cloud-config
users:
- name: "bar"
`)
			Expect(len(yipConfig.Stages)).To(Equal(3))
			Expect(yipConfig.Stages["boot"][0].Users["bar"].Name).To(Equal("bar"))
		})
	})
	Context("YipConfig", Label("schema"), func() {
        // Making sure we bypass this issue:
        // https://github.com/mudler/yip/pull/250/changes#diff-e112952d4a4e1398163b57958ef00de86d89f769005526d7d7d1728de6e75ca0R226
		It("Dumps YipConfig to string and loads it with no issues", func() {
			yipConfig := &YipConfig{
				Stages: map[string][]Stage{
					"test": {
						{
							Name: "Test Stage",
							Files: []File{
								{
									Path:        "/tmp/test.cfg",
									Permissions: 0644,
									Owner:       0,
									Group:       0,
									Content:     fileContent,
								},
							},
						},
					},
				},
			}
			dumped := yipConfig.ToString()

			// Load it back to confirm that dumping it produces a valid yip config
			_ = loadstdYip(dumped)
		})
	})

})

var _ = Describe("FromFile", func() {
	var fs vfs.FS
	var cleanup func()

	// loadWithin runs the load on its own goroutine, because the bug this
	// guards against is a blocking open: a plain call would hang the suite
	// instead of failing it.
	loadWithin := func(path string) error {
		done := make(chan error, 1)
		go func() {
			_, err := Load(path, fs, FromFile, nil)
			done <- err
		}()
		select {
		case err := <-done:
			return err
		case <-time.After(10 * time.Second):
			Fail("Load did not return within 10s, it is blocked on " + path)
			return nil
		}
	}

	rawPath := func(path string) string {
		raw, err := fs.RawPath(path)
		Expect(err).ShouldNot(HaveOccurred())
		return raw
	}

	BeforeEach(func() {
		var err error
		fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{
			"/config/good.yaml": "stages:\n  test:\n  - name: noop\n",
		})
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		cleanup()
	})

	It("reads a regular file", func() {
		config, err := Load("/config/good.yaml", fs, FromFile, nil)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(config.Stages["test"][0].Name).To(Equal("noop"))
	})

	// kairos-io/kairos#4865: a FIFO named like a config blocked open(2) until
	// a writer arrived, which on a real boot is never.
	It("refuses a named pipe instead of blocking on it", func() {
		Expect(syscall.Mkfifo(rawPath("/config/pipe.yaml"), 0o600)).To(Succeed())

		err := loadWithin("/config/pipe.yaml")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("is a named pipe, not a regular file"))
	})

	It("refuses a symlink that points at a named pipe", func() {
		Expect(syscall.Mkfifo(rawPath("/config/pipe"), 0o600)).To(Succeed())
		Expect(fs.Symlink("/config/pipe", "/config/link.yaml")).To(Succeed())

		err := loadWithin("/config/link.yaml")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("is a named pipe, not a regular file"))
	})

	It("still follows a symlink that points at a config", func() {
		Expect(fs.Symlink("/config/good.yaml", "/config/link.yaml")).To(Succeed())

		config, err := Load("/config/link.yaml", fs, FromFile, nil)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(config.Stages["test"][0].Name).To(Equal("noop"))
	})

	It("refuses a directory", func() {
		Expect(fs.Mkdir("/config/dir.yaml", 0o755)).To(Succeed())

		err := loadWithin("/config/dir.yaml")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("is a directory, not a regular file"))
	})
})

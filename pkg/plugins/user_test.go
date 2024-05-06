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
	"fmt"
	"io"

	xpasswd "github.com/mauromorales/xpasswd/pkg/users"
	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v4/vfst"
)

func HaveAllDefaultUsers() types.GomegaMatcher {
	return &HaveAllDefaultUsersMatcher{}
}

type HaveAllDefaultUsersMatcher struct {
	Reason string
}

func (matcher *HaveAllDefaultUsersMatcher) Match(actual interface{}) (bool, error) {
	for _, u := range []string{"root", "bin", "daemon", "mail", "ftp", "http", "systemd-coredump", "systemd-network",
		"systemd-oom", "systemd-journal-remote", "systemd-resolve", "systemd-timesync", "tss", "_talkd", "uuidd",
		"avahi", "named", "colord", "dnsmasq", "gdm", "geoclue", "git", "nm-openconnect", "nm-openvpn", "ntp",
		"openvpn", "polkitd", "rpc", "rpcuser", "rtkit", "usbmux", "nvidia-persistenced", "flatpak", "brltty",
		"gluster", "qemu", "libvirt-qemu", "fwupd", "passim", "cups", "saned", "last",
	} {
		actual := actual.(xpasswd.UserList)

		user := actual.Get(u)
		if user == nil {
			return false, fmt.Errorf("User %s not found", u)
		}
	}
	return true, nil
}

func (matcher *HaveAllDefaultUsersMatcher) FailureMessage(actual interface{}) string {
	if matcher.Reason == "" {
		return format.Message(actual, "to have all default users")
	} else {
		return matcher.Reason
	}
}

func (matcher *HaveAllDefaultUsersMatcher) NegatedFailureMessage(interface{}) string {
	if matcher.Reason == "" {
		return "not to have all default users"
	} else {
		return matcher.Reason
	}
}

var _ = Describe("User", func() {
	Context("parsing yip file", func() {
		testConsole := consoletests.TestConsole{}
		l := logrus.New()
		l.SetOutput(io.Discard)
		existingPasswd := `dbus:x:81:81:System Message Bus:/:/usr/bin/nologin
root:x:0:0::/root:/bin/bash
bin:x:1:1::/:/usr/bin/nologin
daemon:x:2:2::/:/usr/bin/nologin
mail:x:8:12::/var/spool/mail:/usr/bin/nologin
ftp:x:14:11::/srv/ftp:/usr/bin/nologin
http:x:33:33::/srv/http:/usr/bin/nologin
systemd-coredump:x:980:980:systemd Core Dumper:/:/usr/bin/nologin
systemd-network:x:979:979:systemd Network Management:/:/usr/bin/nologin
systemd-oom:x:978:978:systemd Userspace OOM Killer:/:/usr/bin/nologin
systemd-journal-remote:x:977:977:systemd Journal Remote:/:/usr/bin/nologin
systemd-resolve:x:976:976:systemd Resolver:/:/usr/bin/nologin
systemd-timesync:x:975:975:systemd Time Synchronization:/:/usr/bin/nologin
tss:x:974:974:tss user for tpm2:/:/usr/bin/nologin
uuidd:x:68:68::/:/usr/bin/nologin
_talkd:x:973:973:User for legacy talkd server:/:/usr/bin/nologin
avahi:x:972:972:Avahi mDNS/DNS-SD daemon:/:/usr/bin/nologin
named:x:40:40:BIND DNS Server:/:/usr/bin/nologin
colord:x:971:971:Color management daemon:/var/lib/colord:/usr/bin/nologin
dnsmasq:x:970:970:dnsmasq daemon:/:/usr/bin/nologin
gdm:x:120:120:Gnome Display Manager:/var/lib/gdm:/usr/bin/nologin
geoclue:x:969:969:Geoinformation service:/var/lib/geoclue:/usr/bin/nologin
git:x:968:968:git daemon user:/:/usr/bin/git-shell
nm-openconnect:x:967:967:NetworkManager OpenConnect:/:/usr/bin/nologin
nm-openvpn:x:966:966:NetworkManager OpenVPN:/:/usr/bin/nologin
ntp:x:87:87:Network Time Protocol:/var/lib/ntp:/bin/false
openvpn:x:965:965:OpenVPN:/:/usr/bin/nologin
polkitd:x:102:102:PolicyKit daemon:/:/usr/bin/nologin
rpc:x:32:32:Rpcbind Daemon:/var/lib/rpcbind:/usr/bin/nologin
rpcuser:x:34:34:RPC Service User:/var/lib/nfs:/usr/bin/nologin
rtkit:x:133:133:RealtimeKit:/proc:/usr/bin/nologin
usbmux:x:140:140:usbmux user:/:/usr/bin/nologin
nvidia-persistenced:x:143:143:NVIDIA Persistence Daemon:/:/usr/bin/nologin
flatpak:x:964:964:Flatpak system helper:/:/usr/bin/nologin
brltty:x:961:961:Braille Device Daemon:/var/lib/brltty:/usr/bin/nologin
gluster:x:960:960:GlusterFS daemons:/var/run/gluster:/usr/bin/nologin
qemu:x:959:959:QEMU user:/:/usr/bin/nologin
libvirt-qemu:x:957:957:Libvirt QEMU user:/:/usr/bin/nologin
fwupd:x:956:956:Firmware update daemon:/var/lib/fwupd:/usr/bin/nologin
passim:x:955:955:Local Caching Server:/usr/share/empty:/usr/bin/nologin
cups:x:209:209:cups helper user:/:/usr/bin/nologin
saned:x:953:953:SANE daemon user:/:/usr/bin/nologin
last:x:999:999:Test user for uid:/:/usr/bin/nologin
`
		BeforeEach(func() {
			consoletests.Reset()
		})
		It("change user password", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/etc/passwd": existingPasswd,
				"/etc/shadow": "",
				"/etc/group":  "",
			})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = User(l, schema.Stage{
				Users: map[string]schema.User{"foo": {PasswordHash: `$fkekofe`, SSHAuthorizedKeys: []string{"github:mudler", "efafeeafea,t,t,pgl3,pbar"}}},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			shadow, err := fs.ReadFile("/etc/shadow")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(err).ShouldNot(HaveOccurred())
			group, err := fs.ReadFile("/etc/group")
			Expect(err).ShouldNot(HaveOccurred())

			passdRaw, _ := fs.RawPath("/etc/passwd")

			list := xpasswd.NewUserList()
			list.SetPath(passdRaw)
			err = list.Load()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(list).To(HaveAllDefaultUsers())

			Expect(string(group)).Should(Equal("foo:x:1000:foo\n"))

			Expect(string(shadow)).Should(ContainSubstring("foo:$fkekofe:"))
			foo := list.Get("foo")
			Expect(foo).ToNot(BeNil())
			Expect(foo.RealName()).To(Equal("Created by entities"))
			Expect(foo.HomeDir()).To(Equal("/home/foo"))
			Expect(foo.Shell()).To(Equal("/bin/sh"))
			// Last user in the default passwd test data is 999 so this should be 100
			Expect(foo.UID()).To(Equal(1000))

			file, err := fs.Open("/home/foo/.ssh/authorized_keys")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := io.ReadAll(file)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(b)).Should(ContainSubstring("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDR9zjXvyzg1HFMC7RT4LgtR+YGstxWDPPRoAcNrAWjtQcJVrcVo4WLFnT0BMU5mtMxWSrulpC6yrwnt2TE3Ul86yMxO2hbSyGP/xOdYm/nQzufY49rd3tKeJl1+6DkczuPa+XYh1GBcW5E2laNM5ZK+RjABppMpDgmnrM3AsGNE6G8RSuUvc/6Rwt61ma+jak3F5YMj4kwr5PhY2MTPo2YshsL3ouRXP/uPsbaBM6AdQakjWGJR8tPbrnHenzF65813d9zuY4y78TG0AHfomx9btmha7Mc0YF+BpELnvSQLlYrlRY/ziGhP65aQc8lFMc+XBnHeaXF4NHnzq6dIH2D"))
			Expect(string(b)).Should(ContainSubstring("efafeeafea,t,t,pgl3,pbar"))
		})

		It("set UID and Lockpasswd", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/etc/passwd": existingPasswd,
				"/etc/shadow": "",
				"/etc/group":  "",
			})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = User(l, schema.Stage{
				Users: map[string]schema.User{"foo": {
					PasswordHash: `$fkekofe`,
					LockPasswd:   true,
					UID:          "5000",
					Homedir:      "/run/foo",
					Shell:        "/bin/bash",
				}},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			shadow, err := fs.ReadFile("/etc/shadow")
			Expect(err).ShouldNot(HaveOccurred())
			group, err := fs.ReadFile("/etc/group")
			Expect(err).ShouldNot(HaveOccurred())

			passdRaw, _ := fs.RawPath("/etc/passwd")

			list := xpasswd.NewUserList()
			list.SetPath(passdRaw)
			err = list.Load()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(list).To(HaveAllDefaultUsers())

			Expect(string(group)).Should(Equal("foo:x:1000:foo\n"))

			Expect(string(shadow)).Should(ContainSubstring("foo:!:"))
			foo := list.Get("foo")
			Expect(foo).ToNot(BeNil())

			Expect(foo.RealName()).To(Equal("Created by entities"))
			Expect(foo.HomeDir()).To(Equal("/run/foo"))
			Expect(foo.Shell()).To(Equal("/bin/bash"))
			// we specifically set this UID()
			Expect(foo.UID()).To(Equal(5000))

		})

		It("edits already existing user password", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/etc/passwd": existingPasswd,
				"/etc/shadow": `foo:$6$rfBd56ti$7juhxebonsy.GiErzyxZPkbm.U4lUlv/59D2pvFqlbjVqyJP5f4VgP.EX3FKAeGTAr.GVf0jQmy9BXAZL5mNJ1:18820::::::
rancher:$6$2SMtYvSg$wL/zzuT4m3uYkHWO1Rl4x5U6BeGu9IfzIafueinxnNgLFHI34En35gu9evtlhizsOxRJLaTfy0bWFZfm2.qYu1:18820::::::`,
				"/etc/group": "",
			})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = User(l, schema.Stage{
				Users: map[string]schema.User{"foo": {PasswordHash: `$fkekofe`, Homedir: "/home/foo", SSHAuthorizedKeys: []string{"github:mudler", "efafeeafea,t,t,pgl3,pbar"}}},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			shadow, err := fs.ReadFile("/etc/shadow")
			Expect(err).ShouldNot(HaveOccurred())
			group, err := fs.ReadFile("/etc/group")
			Expect(err).ShouldNot(HaveOccurred())

			passdRaw, _ := fs.RawPath("/etc/passwd")

			list := xpasswd.NewUserList()
			list.SetPath(passdRaw)
			err = list.Load()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(list).To(HaveAllDefaultUsers())

			Expect(string(group)).Should(Equal("foo:x:1000:foo\n"))

			Expect(string(shadow)).Should(ContainSubstring("foo:$fkekofe:"))
			foo := list.Get("foo")
			Expect(foo).ToNot(BeNil())

			Expect(foo.RealName()).To(Equal("Created by entities"))
			Expect(foo.HomeDir()).To(Equal("/home/foo"))
			Expect(foo.Shell()).To(Equal("/bin/sh"))
			// first free UID() is 1000
			Expect(foo.UID()).To(Equal(1000))

			file, err := fs.Open("/home/foo/.ssh/authorized_keys")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := io.ReadAll(file)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(b)).Should(ContainSubstring("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDR9zjXvyzg1HFMC7RT4LgtR+YGstxWDPPRoAcNrAWjtQcJVrcVo4WLFnT0BMU5mtMxWSrulpC6yrwnt2TE3Ul86yMxO2hbSyGP/xOdYm/nQzufY49rd3tKeJl1+6DkczuPa+XYh1GBcW5E2laNM5ZK+RjABppMpDgmnrM3AsGNE6G8RSuUvc/6Rwt61ma+jak3F5YMj4kwr5PhY2MTPo2YshsL3ouRXP/uPsbaBM6AdQakjWGJR8tPbrnHenzF65813d9zuY4y78TG0AHfomx9btmha7Mc0YF+BpELnvSQLlYrlRY/ziGhP65aQc8lFMc+XBnHeaXF4NHnzq6dIH2D"))
			Expect(string(b)).Should(ContainSubstring("efafeeafea,t,t,pgl3,pbar"))
		})

		It("adds users to group", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/etc/passwd": existingPasswd,
				"/etc/shadow": ``,
				"/etc/group":  "",
			})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = User(l, schema.Stage{
				Users: map[string]schema.User{"admin": {PasswordHash: `$fkekofe`, Homedir: "/home/foo", SSHAuthorizedKeys: []string{"github:mudler", "efafeeafea,t,t,pgl3,pbar"}}},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			err = User(l, schema.Stage{
				Users: map[string]schema.User{"bar": {Groups: []string{"admin"}, PasswordHash: `$fkekofe`, Homedir: "/home/foo", SSHAuthorizedKeys: []string{"github:mudler", "efafeeafea,t,t,pgl3,pbar"}}},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			group, err := fs.ReadFile("/etc/group")
			Expect(err).ShouldNot(HaveOccurred())

			Expect(string(group)).Should(Equal("admin:x:1000:admin,bar\nbar:x:1001:bar\n"))

			err = User(l, schema.Stage{
				Users: map[string]schema.User{"baz": {Homedir: "/home/foo", Groups: []string{"admin"}}},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			group, err = fs.ReadFile("/etc/group")
			Expect(err).ShouldNot(HaveOccurred())

			Expect(string(group)).Should(Equal("admin:x:1000:admin,bar,baz\nbar:x:1001:bar\nbaz:x:1002:baz\n"))

		})

		It("Recreates users with the same UID() and in order", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/etc/passwd": existingPasswd,
				"/etc/shadow": "",
				"/etc/group":  "",
			})
			Expect(err).Should(BeNil())
			defer cleanup()

			users := map[string]schema.User{
				"foo": {PasswordHash: `$fkekofe`},
				"bar": {PasswordHash: `$fkekofe`},
				"x":   {PasswordHash: `$fkekofe`},
				"a":   {PasswordHash: `$fkekofe`},
			}

			err = User(l, schema.Stage{
				Users: users,
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			passdRaw, _ := fs.RawPath("/etc/passwd")
			list := xpasswd.NewUserList()
			list.SetPath(passdRaw)
			err = list.Load()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(list).To(HaveAllDefaultUsers())

			a := list.Get("a")
			Expect(a).ToNot(BeNil())

			Expect(a.RealName()).To(Equal("Created by entities"))
			Expect(a.HomeDir()).To(Equal("/home/a"))
			Expect(a.Shell()).To(Equal("/bin/sh"))
			// first free UID() is 1000
			Expect(a.UID()).To(Equal(1000))

			bar := list.Get("bar")
			Expect(bar).ToNot(BeNil())

			Expect(bar.RealName()).To(Equal("Created by entities"))
			Expect(bar.HomeDir()).To(Equal("/home/bar"))
			Expect(bar.Shell()).To(Equal("/bin/sh"))
			// Next UID()
			Expect(bar.UID()).To(Equal(1001))

			foo := list.Get("foo")
			Expect(foo).ToNot(BeNil())

			Expect(foo.RealName()).To(Equal("Created by entities"))
			Expect(foo.HomeDir()).To(Equal("/home/foo"))
			Expect(foo.Shell()).To(Equal("/bin/sh"))
			// first free UID() is 1000
			Expect(foo.UID()).To(Equal(1002))

			x := list.Get("x")
			Expect(x).ToNot(BeNil())

			Expect(x.RealName()).To(Equal("Created by entities"))
			Expect(x.HomeDir()).To(Equal("/home/x"))
			Expect(x.Shell()).To(Equal("/bin/sh"))
			// first free UID() is 1000
			Expect(x.UID()).To(Equal(1003))

			// Manual calling cleanup so we start from scratch
			cleanup()

			fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{"/etc/passwd": existingPasswd,
				"/etc/shadow": "",
				"/etc/group":  "",
			})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = User(l, schema.Stage{
				Users: users,
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			passdRaw, _ = fs.RawPath("/etc/passwd")
			list = xpasswd.NewUserList()
			list.SetPath(passdRaw)
			err = list.Load()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(list).To(HaveAllDefaultUsers())

			a = list.Get("a")
			Expect(a).ToNot(BeNil())

			Expect(a.RealName()).To(Equal("Created by entities"))
			Expect(a.HomeDir()).To(Equal("/home/a"))
			Expect(a.Shell()).To(Equal("/bin/sh"))
			// first free UID() is 1000
			Expect(a.UID()).To(Equal(1000))

			bar = list.Get("bar")
			Expect(bar).ToNot(BeNil())

			Expect(bar.RealName()).To(Equal("Created by entities"))
			Expect(bar.HomeDir()).To(Equal("/home/bar"))
			Expect(bar.Shell()).To(Equal("/bin/sh"))
			// Next UID()
			Expect(bar.UID()).To(Equal(1001))

			foo = list.Get("foo")
			Expect(foo).ToNot(BeNil())

			Expect(foo.RealName()).To(Equal("Created by entities"))
			Expect(foo.HomeDir()).To(Equal("/home/foo"))
			Expect(foo.Shell()).To(Equal("/bin/sh"))
			// first free UID() is 1000
			Expect(foo.UID()).To(Equal(1002))

			x = list.Get("x")
			Expect(x).ToNot(BeNil())

			Expect(x.RealName()).To(Equal("Created by entities"))
			Expect(x.HomeDir()).To(Equal("/home/x"))
			Expect(x.Shell()).To(Equal("/bin/sh"))
			// first free UID() is 1000
			Expect(x.UID()).To(Equal(1003))
		})

		It("Creates the user multiple times, keeping the same UID()", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/etc/passwd": existingPasswd,
				"/etc/shadow": "",
				"/etc/group":  "",
			})
			Expect(err).Should(BeNil())
			defer cleanup()

			users := map[string]schema.User{
				"foo": {PasswordHash: `$fkekofe`},
			}

			err = User(l, schema.Stage{
				Users: users,
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			err = User(l, schema.Stage{
				Users: users,
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			err = User(l, schema.Stage{
				Users: users,
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			err = User(l, schema.Stage{
				Users: users,
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			err = User(l, schema.Stage{
				Users: users,
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			passdRaw, _ := fs.RawPath("/etc/passwd")
			list := xpasswd.NewUserList()
			list.SetPath(passdRaw)
			err = list.Load()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(list).To(HaveAllDefaultUsers())

			foo := list.Get("foo")
			Expect(foo).ToNot(BeNil())

			Expect(foo.RealName()).To(Equal("Created by entities"))
			Expect(foo.HomeDir()).To(Equal("/home/foo"))
			Expect(foo.Shell()).To(Equal("/bin/sh"))
			// first free UID() is 1000, should have not changed
			Expect(foo.UID()).To(Equal(1000))
		})

		It("Creates the user multiple times, keeping the same UID(), even if a new users is added", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/etc/passwd": existingPasswd,
				"/etc/shadow": "",
				"/etc/group":  "",
			})
			Expect(err).Should(BeNil())
			defer cleanup()

			users := map[string]schema.User{
				"foo": {PasswordHash: `$fkekofe`},
			}

			err = User(l, schema.Stage{
				Users: users,
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			// Now we add a new user that is created BEFORE the foo users
			// They are created alphabetically btw
			users = map[string]schema.User{
				"a":   {PasswordHash: `$fkekofe`},
				"b":   {PasswordHash: `$fkekofe`},
				"foo": {PasswordHash: `$fkekofe`},
			}
			err = User(l, schema.Stage{
				Users: users,
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			passdRaw, _ := fs.RawPath("/etc/passwd")
			list := xpasswd.NewUserList()
			list.SetPath(passdRaw)
			err = list.Load()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(list).To(HaveAllDefaultUsers())

			foo := list.Get("foo")
			Expect(foo).ToNot(BeNil())
			Expect(foo.RealName()).To(Equal("Created by entities"))
			Expect(foo.HomeDir()).To(Equal("/home/foo"))
			Expect(foo.Shell()).To(Equal("/bin/sh"))
			// first free UID() is 1000, should have not changed even with other new users getting new UID()s
			Expect(foo.UID()).To(Equal(1000))

			a := list.Get("a")
			Expect(a).ToNot(BeNil())

			Expect(a.RealName()).To(Equal("Created by entities"))
			Expect(a.HomeDir()).To(Equal("/home/a"))
			Expect(a.Shell()).To(Equal("/bin/sh"))
			// Should have been created just after our foo user
			Expect(a.UID()).To(Equal(1001))

			b := list.Get("b")
			Expect(b).ToNot(BeNil())

			Expect(b.RealName()).To(Equal("Created by entities"))
			Expect(b.HomeDir()).To(Equal("/home/b"))
			Expect(b.Shell()).To(Equal("/bin/sh"))
			// Should have been created just after our a user
			Expect(b.UID()).To(Equal(1002))

		})
	})
})

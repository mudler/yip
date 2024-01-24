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
	"io"
	"io/ioutil"
	"strings"

	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v4/vfst"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("User", func() {
	Context("parsing yip file", func() {
		testConsole := consoletests.TestConsole{}
		l := logrus.New()
		l.SetOutput(io.Discard)
		BeforeEach(func() {
			consoletests.Reset()
		})
		It("change user password", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/etc/passwd": "",
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
			passwd, err := fs.ReadFile("/etc/passwd")
			Expect(err).ShouldNot(HaveOccurred())
			group, err := fs.ReadFile("/etc/group")
			Expect(err).ShouldNot(HaveOccurred())

			Expect(string(group)).Should(Equal("foo:x:1000:foo\n"))

			Expect(string(shadow)).Should(ContainSubstring("foo:$fkekofe:"))
			Expect(string(passwd)).Should(Equal("foo:x:1000:1000:Created by entities:/home/foo:/bin/sh\n"))

			file, err := fs.Open("/home/foo/.ssh/authorized_keys")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(b)).Should(ContainSubstring("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDR9zjXvyzg1HFMC7RT4LgtR+YGstxWDPPRoAcNrAWjtQcJVrcVo4WLFnT0BMU5mtMxWSrulpC6yrwnt2TE3Ul86yMxO2hbSyGP/xOdYm/nQzufY49rd3tKeJl1+6DkczuPa+XYh1GBcW5E2laNM5ZK+RjABppMpDgmnrM3AsGNE6G8RSuUvc/6Rwt61ma+jak3F5YMj4kwr5PhY2MTPo2YshsL3ouRXP/uPsbaBM6AdQakjWGJR8tPbrnHenzF65813d9zuY4y78TG0AHfomx9btmha7Mc0YF+BpELnvSQLlYrlRY/ziGhP65aQc8lFMc+XBnHeaXF4NHnzq6dIH2D"))
			Expect(string(b)).Should(ContainSubstring("efafeeafea,t,t,pgl3,pbar"))
		})

		It("set UID and Lockpasswd", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/etc/passwd": "",
				"/etc/shadow": "",
				"/etc/group":  "",
			})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = User(l, schema.Stage{
				Users: map[string]schema.User{"foo": {
					PasswordHash: `$fkekofe`,
					LockPasswd:   true,
					UID:          "0",
					Homedir:      "/run/foo",
					Shell:        "/bin/bash",
				}},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			shadow, err := fs.ReadFile("/etc/shadow")
			Expect(err).ShouldNot(HaveOccurred())
			passwd, err := fs.ReadFile("/etc/passwd")
			Expect(err).ShouldNot(HaveOccurred())
			group, err := fs.ReadFile("/etc/group")
			Expect(err).ShouldNot(HaveOccurred())

			Expect(string(group)).Should(Equal("foo:x:1000:foo\n"))

			Expect(string(shadow)).Should(ContainSubstring("foo:!:"))
			Expect(string(passwd)).Should(Equal("foo:x:0:1000:Created by entities:/run/foo:/bin/bash\n"))
		})

		It("edits already existing user password", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/etc/passwd": "",
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
			passwd, err := fs.ReadFile("/etc/passwd")
			Expect(err).ShouldNot(HaveOccurred())
			group, err := fs.ReadFile("/etc/group")
			Expect(err).ShouldNot(HaveOccurred())

			Expect(string(group)).Should(Equal("foo:x:1000:foo\n"))

			Expect(string(shadow)).Should(ContainSubstring("foo:$fkekofe:"))
			Expect(string(passwd)).Should(Equal("foo:x:1000:1000:Created by entities:/home/foo:/bin/sh\n"))

			file, err := fs.Open("/home/foo/.ssh/authorized_keys")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(b)).Should(ContainSubstring("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDR9zjXvyzg1HFMC7RT4LgtR+YGstxWDPPRoAcNrAWjtQcJVrcVo4WLFnT0BMU5mtMxWSrulpC6yrwnt2TE3Ul86yMxO2hbSyGP/xOdYm/nQzufY49rd3tKeJl1+6DkczuPa+XYh1GBcW5E2laNM5ZK+RjABppMpDgmnrM3AsGNE6G8RSuUvc/6Rwt61ma+jak3F5YMj4kwr5PhY2MTPo2YshsL3ouRXP/uPsbaBM6AdQakjWGJR8tPbrnHenzF65813d9zuY4y78TG0AHfomx9btmha7Mc0YF+BpELnvSQLlYrlRY/ziGhP65aQc8lFMc+XBnHeaXF4NHnzq6dIH2D"))
			Expect(string(b)).Should(ContainSubstring("efafeeafea,t,t,pgl3,pbar"))
		})

		It("adds users to group", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/etc/passwd": "",
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

		It("Recreates users with the same UID and in order", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/etc/passwd": "",
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

			passwd, err := fs.ReadFile("/etc/passwd")
			Expect(err).ShouldNot(HaveOccurred())

			passwdLines := strings.Split(string(passwd), "\n")
			Expect(passwdLines[0]).Should(Equal("a:x:1000:1000:Created by entities:/home/a:/bin/sh"))
			Expect(passwdLines[1]).Should(Equal("bar:x:1001:1001:Created by entities:/home/bar:/bin/sh"))
			Expect(passwdLines[2]).Should(Equal("foo:x:1002:1002:Created by entities:/home/foo:/bin/sh"))
			Expect(passwdLines[3]).Should(Equal("x:x:1003:1003:Created by entities:/home/x:/bin/sh"))
			// Manual calling cleanup so we start from scratch
			cleanup()

			fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{"/etc/passwd": "",
				"/etc/shadow": "",
				"/etc/group":  "",
			})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = User(l, schema.Stage{
				Users: users,
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			passwd, err = fs.ReadFile("/etc/passwd")
			Expect(err).ShouldNot(HaveOccurred())

			passwdLines = strings.Split(string(passwd), "\n")
			Expect(passwdLines[0]).Should(Equal("a:x:1000:1000:Created by entities:/home/a:/bin/sh"))
			Expect(passwdLines[1]).Should(Equal("bar:x:1001:1001:Created by entities:/home/bar:/bin/sh"))
			Expect(passwdLines[2]).Should(Equal("foo:x:1002:1002:Created by entities:/home/foo:/bin/sh"))
			Expect(passwdLines[3]).Should(Equal("x:x:1003:1003:Created by entities:/home/x:/bin/sh"))
		})
	})
})

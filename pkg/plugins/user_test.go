// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
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

package plugins_test

import (
	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	"github.com/twpayne/go-vfs/vfst"
	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("User", func() {
	Context("parsing yip file", func() {
		testConsole := consoletests.TestConsole{}
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

			err = User(schema.Stage{
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
			Expect(string(passwd)).Should(Equal("foo:x:1000:1000::/home/foo:\n"))

			file, err := fs.Open("/home/foo/.ssh/authorized_keys")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(b)).Should(Equal("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDR9zjXvyzg1HFMC7RT4LgtR+YGstxWDPPRoAcNrAWjtQcJVrcVo4WLFnT0BMU5mtMxWSrulpC6yrwnt2TE3Ul86yMxO2hbSyGP/xOdYm/nQzufY49rd3tKeJl1+6DkczuPa+XYh1GBcW5E2laNM5ZK+RjABppMpDgmnrM3AsGNE6G8RSuUvc/6Rwt61ma+jak3F5YMj4kwr5PhY2MTPo2YshsL3ouRXP/uPsbaBM6AdQakjWGJR8tPbrnHenzF65813d9zuY4y78TG0AHfomx9btmha7Mc0YF+BpELnvSQLlYrlRY/ziGhP65aQc8lFMc+XBnHeaXF4NHnzq6dIH2D\nssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDjWfZUB5W9HU70yOD1QW/7DSYZsisg8pPHnrxzS5WFnUvhnd7x3r9i+L8mRfk0tXk9p599e5uTryqaHW74bQK360+TnVens0JRF5vGeABe2L2GGrIkTIF8aTlPVq2BTDhu0R0rU28Cw3HwywX7cNjZdpFN2MtF74QbwqB0Ue7Nj6XxJjgV7GcecKEWc23Vjie6KEHlkFcgS0objZsiSt+hY3v3wJ94t+WZ8d1vEwvp7PX2J20W8Zq0bGcJiGMGuhDPRAZ4ju6HxIm60fUo9WzMNrZKVyEbMSYo6frLcmcMN0cDpDXE9WWnCwKDKnZEB0WqQcwOh1TQLYvRYEgMJair\n\nefafeeafea,t,t,pgl3,pbar\n"))
		})
	})
})

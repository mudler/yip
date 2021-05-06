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
				Users: map[string]schema.User{"foo": {PasswordHash: `$fkekofe`, Homedir: "/home/foo"}},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			shadow, err := fs.ReadFile("/etc/shadow")
			Expect(err).ShouldNot(HaveOccurred())
			passwd, err := fs.ReadFile("/etc/passwd")
			Expect(err).ShouldNot(HaveOccurred())
			group, err := fs.ReadFile("/etc/group")
			Expect(err).ShouldNot(HaveOccurred())

			Expect(string(group)).Should(Equal("foo:x:1000:foo\n"))

			Expect(string(shadow)).Should(Equal("foo:$fkekofe:18753::::::\n"))
			Expect(string(passwd)).Should(Equal("foo:x:1000:1000::/home/foo:\n"))
		})
	})
})

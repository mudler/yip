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
	"fmt"
	"io/ioutil"

	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/utils"
	"github.com/twpayne/go-vfs/vfst"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Hostname", func() {
	Context("setting", func() {
		It("configures /etc/hostname", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/etc/hostname": "boo", "/etc/hosts": "127.0.0.1 boo"})
			Expect(err).Should(BeNil())
			defer cleanup()

			ts, err := utils.TemplatedString("bar", nil)
			Expect(err).Should(BeNil())

			err = SystemHostname(ts, fs)
			Expect(err).ShouldNot(HaveOccurred())

			err = UpdateHostsFile(ts, fs)
			Expect(err).ShouldNot(HaveOccurred())

			file, err := fs.Open("/etc/hostname")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(b)).Should(Equal(fmt.Sprintf("%s\n", ts)))

			file, err = fs.Open("/etc/hosts")
			Expect(err).ShouldNot(HaveOccurred())

			b, err = ioutil.ReadAll(file)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(b)).Should(Equal(fmt.Sprintf("127.0.0.1 localhost %s\n", ts)))
		})
	})
})

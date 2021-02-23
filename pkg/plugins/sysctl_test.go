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
	"io/ioutil"

	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	"github.com/twpayne/go-vfs/vfst"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sysctl", func() {
	Context("setting", func() {
		testConsole := consoletests.TestConsole{}

		It("configures a /sys/proc setting", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/proc/sys/debug/.keep": ""})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = Sysctl(schema.Stage{
				Sysctl: map[string]string{"debug.exception-trace": "0"},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			file, err := fs.Open("/proc/sys/debug/exception-trace")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(b)).Should(Equal("0"))
		})
	})
})

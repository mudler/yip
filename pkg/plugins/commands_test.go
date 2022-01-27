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
	"runtime"

	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/vfst"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Commands", func() {
	Context("parsing yip file", func() {
		testConsole := consoletests.TestConsole{}
		l := logrus.New()

		BeforeEach(func() {
			consoletests.Reset()
		})
		It("execute commands", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = Commands(l, schema.Stage{
				Commands: []string{"echo foo", "echo bar"},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(consoletests.Commands).Should(Equal([]string{"echo foo", "echo bar"}))
		})
		It("execute templated commands", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			defer cleanup()
			arch := runtime.GOARCH
			err = Commands(l, schema.Stage{
				Commands: []string{"echo {{.Values.os.architecture}}", "echo bar"},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(consoletests.Commands).Should(Equal([]string{"echo " + arch, "echo bar"}))
		})
	})
})

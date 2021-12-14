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
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/vfst"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SystemdFirstboot", func() {
	Context("parsing yip file", func() {
		testConsole := consoletests.TestConsole{}
		BeforeEach(func() {
			consoletests.Reset()
		})
		It("sets first-boot configuration", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = SystemdFirstboot(logrus.New(), schema.Stage{
				SystemdFirstBoot: map[string]string{
					"keymap": "us",
					"LOCALE": "en_US.UTF-8",
				},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(consoletests.Commands).Should(ContainElements(
				"systemd-firstboot --locale=en_US.UTF-8",
				"systemd-firstboot --keymap=us",
			))
			Expect(len(consoletests.Commands)).To(Equal(2))
		})
	})
})

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
	"os"

	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/vfst"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Environment", func() {
	Context("setting", func() {
		testConsole := consoletests.TestConsole{}
		l := logrus.New()
		It("configures a /etc/environment setting", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/etc/environment": ""})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = Environment(l, schema.Stage{
				Environment: map[string]string{"foo": "0"},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			file, err := fs.Open("/etc/environment")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(b)).Should(Equal("foo=\"0\""))
		})
		It("configures a /run/cos/cos-layout.env file and creates missing directories", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/run": &vfst.Dir{Perm: 0o755}})
			Expect(err).Should(BeNil())
			defer cleanup()

			_, err = fs.Stat("/run/cos")
			Expect(err).NotTo(BeNil())

			err = Environment(l, schema.Stage{
				Environment:     map[string]string{"foo": "0"},
				EnvironmentFile: "/run/cos/cos-layout.env",
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			inf, _ := fs.Stat("/run/cos")
			Expect(inf.Mode().Perm()).To(Equal(os.FileMode(int(0744))))

			file, err := fs.Open("/run/cos/cos-layout.env")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(b)).Should(Equal("foo=\"0\""))
		})
	})
})

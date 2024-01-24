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
	"os"

	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v4/vfst"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Environment", func() {
	Context("setting", func() {
		testConsole := consoletests.TestConsole{}
		l := logrus.New()
		l.SetOutput(io.Discard)
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
			Expect(string(b)).Should(Equal("foo=0\n"))
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
			Expect(string(b)).Should(Equal("foo=0\n"))
		})
	})
})

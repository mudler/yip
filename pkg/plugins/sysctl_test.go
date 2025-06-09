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

	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v4/vfst"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sysctl", func() {
	Context("setting", func() {
		testConsole := consoletests.TestConsole{}
		l := logrus.New()
		l.SetOutput(io.Discard)

		AfterEach(func() {
			testConsole.Reset()
		})

		It("configures a /sys/proc setting", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/proc/sys/debug/.keep": ""})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = Sysctl(l, schema.Stage{
				Sysctl: map[string]string{"debug.exception-trace": "0"},
			}, fs, &testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			file, err := fs.Open("/proc/sys/debug/exception-trace")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(b)).Should(Equal("0"))
		})
	})
})

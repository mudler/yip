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
	"runtime"

	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v4/vfst"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Commands", func() {
	Context("parsing yip file", func() {
		testConsole := consoletests.TestConsole{}
		l := logrus.New()
		l.SetOutput(io.Discard)

		BeforeEach(func() {
			testConsole.Reset()
		})
		It("execute commands", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = Commands(l, schema.Stage{
				Commands: []string{"echo foo", "echo bar"},
			}, fs, &testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(testConsole.Commands).Should(Equal([]string{"echo foo", "echo bar"}))
		})
		It("execute templated commands", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			defer cleanup()
			arch := runtime.GOARCH
			err = Commands(l, schema.Stage{
				Commands: []string{"echo {{.Values.os.architecture}}", "echo bar"},
			}, fs, &testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(testConsole.Commands).Should(Equal([]string{"echo " + arch, "echo bar"}))
		})
	})
})

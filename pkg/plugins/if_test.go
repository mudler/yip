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
	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v4/vfst"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("If", func() {
	Context("Succeeds", func() {
		testConsole := consoletests.TestConsole{}
		BeforeEach(func() {
			consoletests.Reset()
		})
		It("Executes", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/etc/hostname": "boo", "/etc/hosts": "127.0.0.1 boo"})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = IfConditional(logrus.New(), schema.Stage{
				If: "exit 1",
			}, fs, testConsole)

			Expect(err).ShouldNot(HaveOccurred())
			Expect(consoletests.Commands).Should(Equal([]string{"exit 1"}))

		})
	})
})

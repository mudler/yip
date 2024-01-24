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
	"fmt"
	"io/ioutil"

	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/utils"
	"github.com/twpayne/go-vfs/v4/vfst"

	. "github.com/onsi/ginkgo/v2"
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

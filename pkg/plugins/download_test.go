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
	"io/ioutil"
	"log"
	"os"

	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/vfst"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const testURL = "https://gist.githubusercontent.com/mudler/13d2c42fd2cf7fc33cdb8cae6b5bdd57/raw/486ba13e63ae6a272ac6ff59616b6645f4d01813/unittest.txt"

var _ = Describe("Download", func() {
	Context("download a simple file", func() {
		testConsole := consoletests.TestConsole{}
		l := logrus.New()
		It("downloads correctly in the specified location", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/tmp/test/bar": "boo"})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = Download(l, schema.Stage{
				Downloads: []schema.Download{{Path: "/tmp/test/foo", URL: testURL, Permissions: 0777, Owner: os.Getuid(), Group: os.Getgid()}},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			file, err := fs.Open("/tmp/test/foo")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("test"))
		})
		It("downloads correctly in the specified full path", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/tmp/test/bar": "boo"})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = Download(l, schema.Stage{
				Downloads: []schema.Download{{Path: "/tmp/test/", URL: testURL, Permissions: 0777, Owner: os.Getuid(), Group: os.Getgid()}},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			file, err := fs.Open("/tmp/test/unittest.txt")
			Expect(err).ShouldNot(HaveOccurred())
			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("test"))
		})
	})
})

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

var _ = Describe("Files", func() {
	Context("creating", func() {
		testConsole := consoletests.TestConsole{}
		l := logrus.New()
		It("creates a /tmp/test/foo file", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/tmp/test/bar": "boo"})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = EnsureFiles(l, schema.Stage{
				Files: []schema.File{{Path: "/tmp/test/foo", Content: "Test", Permissions: 0777, Owner: os.Getuid(), Group: os.Getgid()}},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			file, err := fs.Open("/tmp/test/foo")
			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("Test"))
		})
		It("creates a /testarea/dir/subdir/foo file and its parent directories", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/testarea": &vfst.Dir{Perm: 0o755}})
			Expect(err).Should(BeNil())
			defer cleanup()
			_, err = fs.Stat("/testarea/dir")
			Expect(err).NotTo(BeNil())
			err = EnsureFiles(l, schema.Stage{
				Files: []schema.File{{Path: "/testarea/dir/subdir/foo", Content: "Test", Permissions: 0640, Owner: os.Getuid(), Group: os.Getgid()}},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			file, err := fs.Open("/testarea/dir/subdir/foo")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}
			inf, _ := fs.Stat("/testarea/dir/subdir")
			Expect(inf.Mode().Perm()).To(Equal(os.FileMode(int(0740))))

			Expect(string(b)).Should(Equal("Test"))
		})
	})
})

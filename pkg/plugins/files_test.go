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

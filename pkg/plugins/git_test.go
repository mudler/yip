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
	"github.com/twpayne/go-vfs/vfst"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Git", func() {
	Context("creating", func() {
		testConsole := consoletests.TestConsole{}

		It("clones a public repo in a path that doesn't exist", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/testarea": &vfst.Dir{Perm: 0o755}})
			Expect(err).Should(BeNil())
			defer cleanup()
			err = Git(schema.Stage{

				Git: schema.Git{
					URL:  "https://gist.github.com/mudler/13d2c42fd2cf7fc33cdb8cae6b5bdd57",
					Path: "/testarea/foo",
				},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			file, err := fs.Open("/testarea/foo/unittest.txt")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("test"))
		})

		It("clones a public repo in a path that does exist but is not a git repo", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/testarea": &vfst.Dir{Perm: 0o755}})
			Expect(err).Should(BeNil())
			defer cleanup()
			err = Git(schema.Stage{

				Git: schema.Git{
					URL:  "https://gist.github.com/mudler/13d2c42fd2cf7fc33cdb8cae6b5bdd57",
					Path: "/testarea",
				},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			file, err := fs.Open("/testarea/unittest.txt")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("test"))
		})

		It("clones a public repo in a path that is already checked out", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/testarea": &vfst.Dir{Perm: 0o755}})
			Expect(err).Should(BeNil())
			defer cleanup()
			err = Git(schema.Stage{

				Git: schema.Git{
					URL:  "https://gist.github.com/mudler/13d2c42fd2cf7fc33cdb8cae6b5bdd57",
					Path: "/testarea",
				},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			fs.WriteFile("/testarea/unittest.txt", []byte("foo"), os.ModePerm)
			file, err := fs.Open("/testarea/unittest.txt")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("foo"))

			err = Git(schema.Stage{

				Git: schema.Git{
					URL:    "https://gist.github.com/mudler/13d2c42fd2cf7fc33cdb8cae6b5bdd57",
					Path:   "/testarea",
					Branch: "master",
				},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			file, err = fs.Open("/testarea/unittest.txt")
			Expect(err).ShouldNot(HaveOccurred())

			b, err = ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("test"))
		})

	})
})

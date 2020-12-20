// Copyright © 2020 Ettore Di Giacinto <mudler@gentoo.org>
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

package executor_test

import (
	"io/ioutil"
	"log"
	"os"

	. "github.com/mudler/yip/pkg/executor"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs/vfst"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Executor", func() {
	Context("Loading entities via yaml", func() {
		def := NewExecutor("default")

		It("Creates files", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/tmp/test/bar": "boo"})
			Expect(err).Should(BeNil())

			defer cleanup()

			config := schema.YipConfig{Stages: map[string][]schema.Stage{
				"foo": []schema.Stage{{
					Commands: []string{},
					Files:    []schema.File{{Path: "/tmp/test/foo", Content: "Test", Permissions: 0777}},
				}},
			}}

			def.Apply("foo", config, fs)
			file, err := fs.Open("/tmp/test/foo")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("Test"))

		})

		It("Creates dirs", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/tmp/test/bar": "boo"})
			Expect(err).Should(BeNil())

			defer cleanup()

			config := schema.YipConfig{Stages: map[string][]schema.Stage{
				"foo": []schema.Stage{{
					Commands:    []string{},
					Directories: []schema.Directory{{Path: "/tmp/boo", Permissions: 0777}},
				}},
			}}

			def.Apply("foo", config, fs)
			_, err = fs.Open("/tmp/boo")

			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Run commands", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/tmp/test/bar": "boo"})
			Expect(err).Should(BeNil())
			temp := fs.TempDir()

			defer cleanup()

			f, _ := os.Create(temp + "/foo")
			f.WriteString("Test")

			config := schema.YipConfig{Stages: map[string][]schema.Stage{
				"foo": []schema.Stage{{
					Commands: []string{"sed -i 's/Test/bar/g' " + temp + "/foo"},
				}},
			}}

			err = def.Apply("foo", config, fs)
			Expect(err).Should(BeNil())
			file, err := os.Open(temp + "/foo")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("bar"))

		})

		It("Set DNS", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/tmp/test/bar": "boo"})
			Expect(err).Should(BeNil())
			temp := fs.TempDir()

			defer cleanup()

			config := schema.YipConfig{
				Stages: map[string][]schema.Stage{
					"foo": []schema.Stage{{
						Dns: schema.DNS{Path: temp + "/foo", Nameservers: []string{"8.8.8.8"}},
					}}},
			}

			def.Apply("foo", config, fs)
			file, err := os.Open(temp + "/foo")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("nameserver 8.8.8.8\n"))
		})
		It("Get Users", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/tmp/test/bar": ""})
			Expect(err).Should(BeNil())
			temp := fs.TempDir()
			f, err := os.Create(temp + "/foo")
			Expect(err).Should(BeNil())
			_, err = f.WriteString("nm-openconnect:x:979:\n")
			Expect(err).Should(BeNil())
			defer cleanup()

			config := schema.YipConfig{
				Stages: map[string][]schema.Stage{
					"foo": []schema.Stage{{
						EnsureEntities: []schema.YipEntity{{
							Path: temp + "/foo",
							Entity: `kind: "group"
group_name: "foo"
password: "xx"
gid: 1
users: "one,two,tree"
`,
						}}}}},
			}
			err = def.Apply("foo", config, fs)
			Expect(err).ShouldNot(HaveOccurred())
			file, err := os.Open(temp + "/foo")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("nm-openconnect:x:979:\nfoo:xx:1:one,two,tree\n"))
		})

		It("Deletes Users", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/tmp/test/bar": ""})
			Expect(err).Should(BeNil())
			temp := fs.TempDir()
			f, err := os.Create(temp + "/foo")
			Expect(err).Should(BeNil())
			_, err = f.WriteString("nm-openconnect:x:979:\nfoo:xx:1:one,two,tree\n")
			Expect(err).Should(BeNil())
			defer cleanup()

			config := schema.YipConfig{
				Stages: map[string][]schema.Stage{
					"foo": []schema.Stage{{
						DeleteEntities: []schema.YipEntity{{
							Path: temp + "/foo",
							Entity: `kind: "group"
group_name: "foo"
password: "xx"
gid: 1
users: "one,two,tree"
`,
						}}}}}}
			err = def.Apply("foo", config, fs)
			Expect(err).ShouldNot(HaveOccurred())
			file, err := os.Open(temp + "/foo")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("nm-openconnect:x:979:\n"))
		})
	})
})

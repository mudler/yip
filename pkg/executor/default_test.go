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

	"github.com/mudler/yip/pkg/console"

	. "github.com/mudler/yip/pkg/executor"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	"github.com/twpayne/go-vfs/vfst"
	"github.com/zcalusic/sysinfo"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Executor", func() {
	Context("Loading entities via yaml", func() {
		def := NewExecutor("default")
		testConsole := consoletests.TestConsole{}

		It("Interpolates sys info", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/tmp/test/bar": "boo"})
			Expect(err).Should(BeNil())

			defer cleanup()

			config := schema.YipConfig{Stages: map[string][]schema.Stage{
				"foo": {{
					Commands: []string{},
					Files:    []schema.File{{Path: "/tmp/test/foo", Content: "{{.Values.node.hostname}}", Permissions: 0777}},
				}},
			}}

			def.Apply("foo", config, fs, testConsole)
			file, err := fs.Open("/tmp/test/foo")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}
			var si sysinfo.SysInfo
			si.GetSysInfo()
			Expect(string(b)).Should(Equal(si.Node.Hostname))
		})

		It("Filter command node execution", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/tmp/test/bar": "boo"})
			Expect(err).Should(BeNil())
			var si sysinfo.SysInfo
			si.GetSysInfo()
			defer cleanup()

			config := schema.YipConfig{Stages: map[string][]schema.Stage{
				"foo": []schema.Stage{{
					Commands: []string{},
					Files:    []schema.File{{Path: "/tmp/test/foo", Content: "{{.Values.node.hostname}}", Permissions: 0777}},
					Node:     si.Node.Hostname,
				}},
			}}

			def.Apply("foo", config, fs, testConsole)
			file, err := fs.Open("/tmp/test/foo")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal(si.Node.Hostname))

			config = schema.YipConfig{Stages: map[string][]schema.Stage{
				"foo": []schema.Stage{{
					Commands: []string{},
					Files:    []schema.File{{Path: "/tmp/test/bbb", Content: "{{.Values.node.hostname}}", Permissions: 0777}},
					Node:     "barz",
				}},
			}}

			def.Apply("foo", config, fs, testConsole)
			_, err = fs.Open("/tmp/test/bbb")
			Expect(err).Should(HaveOccurred())
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

			def.Apply("foo", config, fs, testConsole)
			_, err = fs.Open("/tmp/boo")

			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Run commands", func() {
			testConsole := console.StandardConsole{}

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

			err = def.Apply("foo", config, fs, testConsole)
			Expect(err).Should(BeNil())
			file, err := os.Open(temp + "/foo")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("bar"))

		})

		It("Run yip files in sequence", func() {
			testConsole := console.StandardConsole{}

			fs2, cleanup2, err := vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			temp := fs2.TempDir()

			defer cleanup2()

			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{
				"/some/yip/01_first.yaml": `
stages:
  test:
  - commands:
    - sed -i 's/boo/bar/g' ` + temp + `/tmp/test/bar
`,
				"/some/yip/02_second.yaml": `
stages:
  test:
  - commands:
    - sed -i 's/bar/baz/g' ` + temp + `/tmp/test/bar
`,
			})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = fs2.Mkdir("/tmp", os.ModePerm)
			Expect(err).Should(BeNil())
			err = fs2.Mkdir("/tmp/test", os.ModePerm)
			Expect(err).Should(BeNil())

			err = fs2.WriteFile("/tmp/test/bar", []byte(`boo`), os.ModePerm)
			Expect(err).Should(BeNil())

			err = def.Walk("test", []string{"/some/yip"}, fs, testConsole)
			Expect(err).Should(BeNil())
			file, err := os.Open(temp + "/tmp/test/bar")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("baz"))

		})

		It("Reports error, and executes all yip files", func() {
			testConsole := console.StandardConsole{}

			fs2, cleanup2, err := vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			temp := fs2.TempDir()

			defer cleanup2()

			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{
				"/some/yip/01_first.yaml": `
stages:
  test:
  - commands:
    - exit 1
`,
				"/some/yip/02_second.yaml": `
stages:
  test:
  - commands:
    - sed -i 's/boo/bar/g' ` + temp + `/tmp/test/bar
`,
			})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = fs2.Mkdir("/tmp", os.ModePerm)
			Expect(err).Should(BeNil())
			err = fs2.Mkdir("/tmp/test", os.ModePerm)
			Expect(err).Should(BeNil())

			err = fs2.WriteFile("/tmp/test/bar", []byte(`boo`), os.ModePerm)
			Expect(err).Should(BeNil())

			err = def.Walk("test", []string{"/some/yip"}, fs, testConsole)
			Expect(err).Should(HaveOccurred())
			file, err := os.Open(temp + "/tmp/test/bar")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("bar"))
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
			err = def.Apply("foo", config, fs, testConsole)
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
			err = def.Apply("foo", config, fs, testConsole)
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

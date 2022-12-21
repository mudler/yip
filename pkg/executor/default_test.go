//   Copyright 2020 Ettore Di Giacinto <mudler@mocaccino.org>
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

package executor_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/mudler/yip/pkg/console"
	"github.com/sirupsen/logrus"

	. "github.com/mudler/yip/pkg/executor"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	"github.com/twpayne/go-vfs/vfst"
	"github.com/zcalusic/sysinfo"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Executor", func() {
	Context("Loading entities via yaml", func() {
		l := logrus.New()
		l.SetOutput(io.Discard)
		def := NewExecutor(WithLogger(l))
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
			Expect(err).ShouldNot(HaveOccurred())

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
			testConsole := console.NewStandardConsole()

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
			testConsole := console.NewStandardConsole()

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

			err = def.Run("test", fs, testConsole, "/some/yip")
			Expect(err).Should(BeNil())
			file, err := os.Open(temp + "/tmp/test/bar")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("baz"))

		})

		It("Run yip files in sequence with after", func() {
			testConsole := console.NewStandardConsole()

			fs2, cleanup2, err := vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			temp := fs2.TempDir()

			defer cleanup2()

			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{
				"/some/yip/01_first.yaml": `
stages:
  test:
  - after: 
    - name: "test.test"
    commands:
    - sed -i 's/bar/baz/g' ` + temp + `/tmp/test/bar
`,
				"/some/yip/02_second.yaml": `
name: "test"
stages:
  test:
  - name: "test"
    commands:
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

			err = def.Run("test", fs, testConsole, "/some/yip")
			Expect(err).Should(BeNil())
			file, err := os.Open(temp + "/tmp/test/bar")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("baz"))

		})

		It("Execute single yip files", func() {
			testConsole := console.NewStandardConsole()

			fs2, cleanup2, err := vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			temp := fs2.TempDir()

			defer cleanup2()

			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{
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

			err = def.Run("test", fs, testConsole, "/some/yip/02_second.yaml")
			Expect(err).ShouldNot(HaveOccurred())
			file, err := os.Open(temp + "/tmp/test/bar")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("bar"), string(b))
		})

		It("Reports error, and executes all yip files", func() {
			testConsole := console.NewStandardConsole()

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

			err = def.Run("test", fs, testConsole, "/some/yip")
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

		It("Skip with if conditionals", func() {
			testConsole := console.NewStandardConsole()

			fs2, cleanup2, err := vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			temp := fs2.TempDir()

			defer cleanup2()

			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{
				"/some/yip/01_first.yaml": `
stages:
  test:
  - commands:
    - echo "bar" > ` + temp + `/tmp/test/bar
`,
				"/some/yip/02_second.yaml": `
stages:
  test:
  - if: "cat ` + temp + `/tmp/test/bar | grep bar"
    commands:
    - echo "baz" > ` + temp + `/tmp/test/baz
`, "/some/yip/03_second.yaml": `
stages:
  test:
  - if: "cat ` + temp + `/tmp/test/baz | grep bar"
    commands:
    - echo "nope" > ` + temp + `/tmp/test/nope
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

			err = def.Run("test", fs, testConsole, "/some/yip")
			Expect(err).Should(BeNil())
			file, err := os.Open(temp + "/tmp/test/baz")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("baz\n"))

			_, err = os.Open(temp + "/tmp/test/nope")
			Expect(err).Should(HaveOccurred())
		})

		It("has multiple instructions", func() {
			testConsole := console.NewStandardConsole()

			fs2, cleanup2, err := vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			temp := fs2.TempDir()

			defer cleanup2()

			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{
				"/some/yip/01_first.yaml": `
name: "Rootfs Layout Settings"
stages:
    rootfs.before:
    - name: "before rootds"
      commands:
      - echo "rootfs.before" >> ` + temp + `/tmp/test/bar
    rootfs:
    - name: "rootfs"
      commands:
      - echo "rootfs" >> ` + temp + `/tmp/test/bar
    - name: "rootfs 2"
      commands:
      - echo "2" >> ` + temp + `/tmp/test/bar
    initramfs:
    - name: "initramfs"
      commands:
      - echo "initramfs" >> ` + temp + `/tmp/test/bar
`,
			})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = fs2.Mkdir("/tmp", os.ModePerm)
			Expect(err).Should(BeNil())
			err = fs2.Mkdir("/tmp/test", os.ModePerm)
			Expect(err).Should(BeNil())

			err = fs2.WriteFile("/tmp/test/bar", []byte(``), os.ModePerm)
			Expect(err).Should(BeNil())

			err = def.Run("rootfs.before", fs, testConsole, "/some/yip")
			Expect(err).Should(BeNil())
			err = def.Run("rootfs", fs, testConsole, "/some/yip")
			Expect(err).Should(BeNil())
			err = def.Run("initramfs", fs, testConsole, "/some/yip")
			Expect(err).Should(BeNil())

			file, err := os.Open(temp + "/tmp/test/bar")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("rootfs.before\nrootfs\n2\ninitramfs\n"))
		})
		It("has multiple instructions in different files", func() {
			testConsole := console.NewStandardConsole()

			fs2, cleanup2, err := vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			temp := fs2.TempDir()

			defer cleanup2()

			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{
				"/some/yip/01_first.yaml": `
name: "Rootfs Layout Settings"
stages:
    rootfs.before:
    - name: "before roots"
      commands:
      - echo "rootfs.before" >> ` + temp + `/tmp/test/bar
    rootfs:
    - name: "rootfs"
      commands:
      - echo "rootfs" >> ` + temp + `/tmp/test/bar
    - name: "rootfs 2"
      commands:
      - echo "2" >> ` + temp + `/tmp/test/bar
    initramfs:
    - name: "initramfs"
      commands:
      - echo "initramfs" >> ` + temp + `/tmp/test/bar
`,
				"/some/yip/02_second.yaml": `
name: "second Rootfs Layout Settings"
stages:
    rootfs.before:
    - name: "second before roots"
      commands:
      - echo "second.rootfs.before" >> ` + temp + `/tmp/test/bar
    rootfs:
    - name: "second rootfs"
      commands:
      - echo "second.rootfs" >> ` + temp + `/tmp/test/bar
    - name: "second rootfs 2"
      commands:
      - echo "second.2" >> ` + temp + `/tmp/test/bar
    initramfs:
    - name: "second initramfs"
      commands:
      - echo "second.initramfs" >> ` + temp + `/tmp/test/bar
`,
			})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = fs2.Mkdir("/tmp", os.ModePerm)
			Expect(err).Should(BeNil())
			err = fs2.Mkdir("/tmp/test", os.ModePerm)
			Expect(err).Should(BeNil())

			err = fs2.WriteFile("/tmp/test/bar", []byte(``), os.ModePerm)
			Expect(err).Should(BeNil())

			g, err := def.Graph("rootfs.before", fs, testConsole, "/some/yip")
			Expect(err).Should(BeNil())

			Expect(len(g)).To(Equal(3), fmt.Sprintf("%+v", g))
			Expect(len(g[1])).To(Equal(1))
			Expect(len(g[2])).To(Equal(1))
			Expect(g[1][0].Name).To(Equal("Rootfs Layout Settings.before roots"))
			Expect(g[2][0].Name).To(Equal("second Rootfs Layout Settings.second before roots"))

			g1, err := def.Graph("rootfs", fs, testConsole, "/some/yip")
			Expect(err).Should(BeNil())
			Expect(len(g1)).To(Equal(5), fmt.Sprintf("%+v", g1))
			Expect(len(g1[1])).To(Equal(1))
			Expect(len(g1[2])).To(Equal(1))
			Expect(g1[1][0].Name).To(Equal("Rootfs Layout Settings.rootfs"))
			Expect(g1[2][0].Name).To(Equal("Rootfs Layout Settings.rootfs 2"))
			Expect(g1[3][0].Name).To(Equal("second Rootfs Layout Settings.second rootfs"))
			Expect(g1[4][0].Name).To(Equal("second Rootfs Layout Settings.second rootfs 2"))

			err = def.Run("rootfs.before", fs, testConsole, "/some/yip")
			Expect(err).Should(BeNil())
			err = def.Run("rootfs", fs, testConsole, "/some/yip")
			Expect(err).Should(BeNil())
			err = def.Run("initramfs", fs, testConsole, "/some/yip")
			Expect(err).Should(BeNil())

			file, err := os.Open(temp + "/tmp/test/bar")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("rootfs.before\nsecond.rootfs.before\nrootfs\n2\nsecond.rootfs\nsecond.2\ninitramfs\nsecond.initramfs\n"), string(b))
		})
	})
})

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
	"os"

	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v4/vfst"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Files", func() {
	Context("creating", func() {
		testConsole := consoletests.TestConsole{}
		l := logrus.New()
		l.SetOutput(io.Discard)
		It("Creates a /tmp/dir directory", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/tmp": &vfst.Dir{Perm: 0o755}})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = EnsureDirectories(l, schema.Stage{
				Directories: []schema.Directory{{Path: "/tmp/dir", Permissions: 0740, Owner: os.Getuid(), Group: os.Getgid()}},
			}, fs, &testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			inf, _ := fs.Stat("/tmp/dir")
			Expect(inf.Mode().Perm()).To(Equal(os.FileMode(int(0740))))
		})

		It("Changes permissions of existing directory /tmp/dir directory", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/tmp/dir": &vfst.Dir{Perm: 0o755}})
			Expect(err).Should(BeNil())
			defer cleanup()
			inf, _ := fs.Stat("/tmp/dir")
			Expect(inf.Mode().Perm()).To(Equal(os.FileMode(int(0755))))
			err = EnsureDirectories(l, schema.Stage{
				Directories: []schema.Directory{{Path: "/tmp/dir", Permissions: 0740, Owner: os.Getuid(), Group: os.Getgid()}},
			}, fs, &testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			inf, _ = fs.Stat("/tmp/dir")
			Expect(inf.Mode().Perm()).To(Equal(os.FileMode(int(0740))))
		})

		It("Creates /tmp/dir/subdir1/subdir2 directory and its missing parent dirs", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/tmp": &vfst.Dir{Perm: 0o755}})
			Expect(err).Should(BeNil())
			defer cleanup()
			err = EnsureDirectories(l, schema.Stage{
				Directories: []schema.Directory{{Path: "/tmp/dir/subdir1/subdir2", Permissions: 0740, Owner: os.Getuid(), Group: os.Getgid()}},
			}, fs, &testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			inf, _ := fs.Stat("/tmp")
			Expect(inf.Mode().Perm()).To(Equal(os.FileMode(int(0755))))
			inf, _ = fs.Stat("/tmp/dir/subdir1/subdir2")
			Expect(inf.Mode().Perm()).To(Equal(os.FileMode(int(0740))))
		})
	})
})

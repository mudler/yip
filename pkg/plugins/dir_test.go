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
	"os"

	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/vfst"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Files", func() {
	Context("creating", func() {
		testConsole := consoletests.TestConsole{}
		l := logrus.New()
		It("Creates a /tmp/dir directory", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/tmp": &vfst.Dir{Perm: 0o755}})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = EnsureDirectories(l, schema.Stage{
				Directories: []schema.Directory{{Path: "/tmp/dir", Permissions: 0740, Owner: os.Getuid(), Group: os.Getgid()}},
			}, fs, testConsole)
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
			}, fs, testConsole)
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
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			inf, _ := fs.Stat("/tmp")
			Expect(inf.Mode().Perm()).To(Equal(os.FileMode(int(0755))))
			inf, _ = fs.Stat("/tmp/dir/subdir1/subdir2")
			Expect(inf.Mode().Perm()).To(Equal(os.FileMode(int(0740))))
		})
	})
})

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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const gitlabKey string = `# gitlab.com:22 SSH-2.0-OpenSSH_7.9p1 Debian-10+deb10u2
# gitlab.com:22 SSH-2.0-OpenSSH_7.9p1 Debian-10+deb10u2
gitlab.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCsj2bNKTBSpIYDEGk9KxsGh3mySTRgMtXL583qmBpzeQ+jqCMRgBqB98u3z++J1sKlXHWfM9dyhSevkMwSbhoR8XIq/U0tCNyokEi/ueaBMCvbcTHhO7FcwzY92WK4Yt0aGROY5qX2UKSeOvuP4D6TPqKF1onrSzH9bx9XUf2lEdWT/ia1NEKjunUqu1xOB/StKDHMoX4/OKyIzuS0q/T1zOATthvasJFoPrAjkohTyaDUz2LN5JoH839hViyEG82yB+MjcFV5MU3N1l1QL3cVUCh93xSaua1N85qivl+siMkPGbO5xR/En4iEY6K2XPASUEMaieWVNTRCtJ4S8H+9
# gitlab.com:22 SSH-2.0-OpenSSH_7.9p1 Debian-10+deb10u2
gitlab.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBFSMqzJeV9rUzU4kWitGjeR4PWSa29SPqJ1fVkhtj3Hw9xjLVXVYrU9QlYWrOLXBpQ6KWjbjTDTdDkoohFzgbEY=
# gitlab.com:22 SSH-2.0-OpenSSH_7.9p1 Debian-10+deb10u2
`

const testPrivateKey string = `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACBbaeOI9ZJluGPUKqsRVlEc1LHXiUr6HYdvzYuKcHSxuQAAAJBpIXkKaSF5
CgAAAAtzc2gtZWQyNTUxOQAAACBbaeOI9ZJluGPUKqsRVlEc1LHXiUr6HYdvzYuKcHSxuQ
AAAEADUKTRroHZj3rJTDbisFNt2/dZs0QQ5mIwNiIYGVFZOltp44j1kmW4Y9QqqxFWURzU
sdeJSvodh2/Ni4pwdLG5AAAACTxjb21tZW50PgECAwQ=
-----END OPENSSH PRIVATE KEY-----
`

var _ = Describe("Git", func() {
	Context("creating", func() {
		testConsole := consoletests.TestConsole{}
		l := logrus.New()
		It("clones a public repo in a path that doesn't exist", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/testarea": &vfst.Dir{Perm: 0o755}})
			Expect(err).Should(BeNil())
			defer cleanup()
			err = Git(l, schema.Stage{

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
			err = Git(l, schema.Stage{

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
			err = Git(l, schema.Stage{
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

			err = Git(l, schema.Stage{
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

		PIt("clones a private repo in a path that is already checked out", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/testarea": &vfst.Dir{Perm: 0o755}})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = Git(l, schema.Stage{
				Git: schema.Git{
					URL:    "git@gitlab.com:mudler/unit-test-repo.git",
					Path:   "/testarea",
					Branch: "main",

					Auth: schema.Auth{PrivateKey: testPrivateKey, PublicKey: gitlabKey},
				},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			fs.WriteFile("/testarea/test.txt", []byte("foo"), os.ModePerm)
			file, err := fs.Open("/testarea/test.txt")
			Expect(err).ShouldNot(HaveOccurred())

			b, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("foo"))

			err = Git(l, schema.Stage{

				Git: schema.Git{
					URL:    "git@gitlab.com:mudler/unit-test-repo.git",
					Path:   "/testarea",
					Branch: "main",
				},
			}, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())

			file, err = fs.Open("/testarea/test.txt")
			Expect(err).ShouldNot(HaveOccurred())

			b, err = ioutil.ReadAll(file)
			if err != nil {
				log.Fatal(err)
			}

			Expect(string(b)).Should(Equal("test\n"))
		})
	})
})

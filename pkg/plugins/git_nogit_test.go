//go:build nogit && !gitbinary

package plugins_test

import (
	"io"

	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v4/vfst"

	. "github.com/mudler/yip/pkg/plugins"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Git", func() {
	Context("creating", func() {
		testConsole := consoletests.TestConsole{}
		l := logrus.New()
		l.SetOutput(io.Discard)
		It("returns a not supported error", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/testarea": &vfst.Dir{Perm: 0o755}})
			Expect(err).Should(BeNil())
			defer cleanup()
			err = Git(l, schema.Stage{

				Git: schema.Git{
					URL:  "https://gist.github.com/mudler/13d2c42fd2cf7fc33cdb8cae6b5bdd57",
					Path: "/testarea/foo",
				},
			}, fs, &testConsole)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("git plugin not available in nogit build"))
		})
	})
})

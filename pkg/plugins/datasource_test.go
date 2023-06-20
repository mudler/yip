//   Copyright 2023 Ettore Di Giacinto <mudler@mocaccino.org>
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
	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/linuxkit/providers"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/vfst"
	"strings"
	"time"
)

var _ = Describe("Datasources", func() {
	Context("running", func() {
		testConsole := consoletests.TestConsole{}
		l := logrus.New()
		l.SetLevel(logrus.DebugLevel)
		//l.SetOutput(io.Discard)
		It("Runs datasources and fails to ", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			defer cleanup()
			err = DataSources(l, schema.Stage{
				DataSources: schema.DataSource{
					Providers: []string{"cdrom"},
				},
			}, fs, testConsole)
			Expect(err).To(HaveOccurred())
			Expect(strings.ToLower(err.Error())).To(ContainSubstring("no metadata/userdata found"))
			_, err = fs.Stat(providers.ConfigPath)
			Expect(err).ShouldNot(HaveOccurred())
		})
		It("Runs each datasource just once", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			defer cleanup()
			prv := []string{"aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws",
				"aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws",
				"aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws",
				"aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws",
				"aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws",
				"aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws",
				"aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws",
				"aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws",
				"aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws",
				"aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws",
				"aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws",
				"aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws"}
			start := time.Now()
			err = DataSources(l, schema.Stage{
				DataSources: schema.DataSource{
					Providers: prv,
				},
			}, fs, testConsole)
			elapsed := time.Since(start)
			Expect(err).To(HaveOccurred())
			Expect(strings.ToLower(err.Error())).To(ContainSubstring("no metadata/userdata found"))
			// check if it took less than 10 seconds. If we were to run all those datasources one after the other
			// it would take much more
			Expect(elapsed).To(BeNumerically("<", 10*time.Second))
			_, err = fs.Stat(providers.ConfigPath)
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})

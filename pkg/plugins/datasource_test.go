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
	"github.com/mudler/yip/pkg/plugins/datasourceProviders"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v4/vfst"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var _ = Describe("Datasources", func() {
	Context("running", func() {
		testConsole := consoletests.TestConsole{}
		l := logrus.New()
		l.SetLevel(logrus.DebugLevel)
		l.SetOutput(io.Discard)
		It("Runs datasources and fails to adquire any metadata", func() {
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
			prv := []string{"vmware", "hetzner", "gcp", "scaleway", "vultr", "digitalocean", "metaldata", "azure", "openstack", "cdrom",
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
				"aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws", "aws"}
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
		It("Properly finds a datasource and transforms it into a userdata file", func() {
			cloudConfigData := "#cloud-config\nhostname: test"
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/oem": ""})
			Expect(err).ToNot(HaveOccurred())
			defer cleanup()
			temp, err := os.MkdirTemp("", "yip-xxx")
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(filepath.Join(temp, "datasource"), []byte(cloudConfigData), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())
			err = DataSources(l, schema.Stage{
				DataSources: schema.DataSource{
					Providers: []string{"file"},
					// This is the path that the file datasource is using. It doesn't use any vfs passed, so it checks the real os fs
					Path: filepath.Join(temp, "datasource"),
				},
			}, fs, testConsole)
			Expect(err).ToNot(HaveOccurred())
			// Final userdata its set on the test fs
			_, err = fs.Stat(filepath.Join(providers.ConfigPath, "userdata.yaml"))
			Expect(err).ToNot(HaveOccurred())
			file, err := fs.ReadFile(filepath.Join(providers.ConfigPath, "userdata.yaml"))
			Expect(err).ToNot(HaveOccurred())
			// Data should match in the file
			Expect(string(file)).To(Equal(cloudConfigData))
		})
		It("Properly decodes VMWARE datasource", func() {
			vmwareData := []byte(`Content-Type: multipart/mixed; boundary="MIMEBOUNDARY"
MIME-Version: 1.0

--MIMEBOUNDARY
Content-Transfer-Encoding: 7bit
Content-Type: text/cloud-config
Mime-Version: 1.0

#cloud-config
hostname: test

--MIMEBOUNDARY
Content-Transfer-Encoding: 7bit
Content-Type: text/x-shellscript
Mime-Version: 1.0

#!/usr/bin/env bash

echo "hi"

--MIMEBOUNDARY--
`)
			ccData := []byte("#cloud-config\nhostname: test\n")
			vmwareCC := DecodeMultipartVmware(vmwareData)
			normalCC := DecodeMultipartVmware(ccData)
			// In both cases the end results should be the same cloud config
			Expect(vmwareCC).To(Equal(ccData))
			Expect(normalCC).To(Equal(ccData))
		})
	})
})

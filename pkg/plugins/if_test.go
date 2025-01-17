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
	"fmt"
	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v4/vfst"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Conditionals", Label("conditionals"), func() {
	var testConsole consoletests.TestConsole
	var fs *vfst.TestFS
	var cleanup func()
	var err error

	BeforeEach(func() {
		testConsole = consoletests.TestConsole{}
		fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{"/etc/hostname": "boo", "/etc/hosts": "127.0.0.1 boo"})
		Expect(err).Should(BeNil())
	})
	AfterEach(func() {
		consoletests.Reset()
		cleanup()
	})
	Describe("IfConditional", func() {
		Context("Succeeds", func() {
			It("Executes", func() {
				err = IfConditional(logrus.New(), schema.Stage{
					If: "exit 1",
				}, fs, testConsole)

				Expect(err).ShouldNot(HaveOccurred())
				Expect(consoletests.Commands).Should(Equal([]string{"exit 1"}))
			})
		})
	})
	Describe("IfOsConditional", func() {
		It("Executes", func() {
			err = OnlyIfOS(logrus.New(), schema.Stage{
				OnlyIfOs: "weird",
			}, fs, testConsole)

			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(fmt.Sprintf(SkipOnlyOs, "weird")))
			Expect(err.Error()).Should(ContainSubstring("doesn't match os name"))
		})
	})
	Describe("IfOsVersionConditional", func() {
		It("Executes", func() {
			err = OnlyIfOSVersion(logrus.New(), schema.Stage{
				OnlyIfOsVersion: "weird",
			}, fs, testConsole)

			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(fmt.Sprintf(SkipOnlyOsVersion, "weird")), err.Error())
		})
	})
	Describe("IfArchConditional", func() {
		It("Fails with no match", func() {
			err = IfArch(logrus.New(), schema.Stage{
				OnlyIfArch: "weird",
			}, fs, testConsole)

			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(fmt.Sprintf(SkipOnlyArch, runtime.GOARCH, "weird")), err.Error())
		})
		It("Succeeds", func() {
			err = IfArch(logrus.New(), schema.Stage{
				OnlyIfArch: runtime.GOARCH,
			}, fs, testConsole)

			Expect(err).ShouldNot(HaveOccurred())
		})
	})
	Describe("IfServiceConditional", func() {
		It("Fails if not supported", func() {
			err = IfServiceManager(logrus.New(), schema.Stage{
				OnlyIfServiceManager: "weird",
			}, fs, testConsole)

			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(fmt.Sprintf(SkipNotSupportedServiceManager, "weird")))
		})
		It("Fails if not matched", func() {
			err = IfServiceManager(logrus.New(), schema.Stage{
				OnlyIfServiceManager: "openrc",
			}, fs, testConsole)

			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(fmt.Sprintf(SkipOnlyServiceManager, "openrc")))
		})
		It("Fails if it finds both", func() {
			// Create our fake systemctl and openrc
			Expect(fs.Mkdir("/sbin", 0755)).ToNot(HaveOccurred())
			Expect(fs.WriteFile("/sbin/systemctl", []byte{}, 0755)).ToNot(HaveOccurred())
			Expect(fs.WriteFile("/sbin/openrc", []byte{}, 0755)).ToNot(HaveOccurred())

			err = IfServiceManager(logrus.New(), schema.Stage{
				OnlyIfServiceManager: "systemd",
			}, fs, testConsole)

			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(SkipBothServices))
		})
		It("Succeeds to find systemctl", func() {
			// Create our fake systemctl
			Expect(fs.Mkdir("/sbin", 0755)).ToNot(HaveOccurred())
			Expect(fs.WriteFile("/sbin/systemctl", []byte{}, 0755)).ToNot(HaveOccurred())

			err = IfServiceManager(logrus.New(), schema.Stage{
				OnlyIfServiceManager: "systemd",
			}, fs, testConsole)

			Expect(err).ShouldNot(HaveOccurred())
		})
		It("Succeeds to find openrc", func() {
			// Create our fake openrc
			Expect(fs.Mkdir("/sbin", 0755)).ToNot(HaveOccurred())
			Expect(fs.WriteFile("/sbin/openrc", []byte{}, 0755)).ToNot(HaveOccurred())

			err = IfServiceManager(logrus.New(), schema.Stage{
				OnlyIfServiceManager: "openrc",
			}, fs, testConsole)

			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})

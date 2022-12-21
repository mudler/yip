package plugins_test

import (
	"io"
	"os"

	"github.com/mudler/yip/pkg/executor"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/vfst"
	"gopkg.in/ini.v1"
)

var _ = Describe("NetworkManager", func() {
	Context("connections", func() {
		l := logrus.New()
		l.SetOutput(io.Discard)
		def := executor.NewExecutor(executor.WithLogger(l))
		testConsole := consoletests.TestConsole{}
		It("Creates connections", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/tmp/test/bar": ""})
			Expect(err).Should(BeNil())
			err = fs.Mkdir("/etc/", os.ModeDir|os.ModePerm)
			Expect(err).Should(BeNil())
			err = fs.Mkdir("/etc/NetworkManager/", os.ModeDir|os.ModePerm)
			Expect(err).Should(BeNil())
			err = fs.Mkdir("/etc/NetworkManager/system-connections/", os.ModeDir|os.ModePerm)
			Expect(err).Should(BeNil())
			defer cleanup()

			config := schema.YipConfig{
				Stages: map[string][]schema.Stage{
					"foo": []schema.Stage{{
						NetworkManager: []schema.NetConnection{
							{
								Name: "test1",
								Connection: []map[string]string{
									{
										"interface-name": "eth2",
										"a":              "b",
									},
								},
								Wifi: []map[string]string{
									{
										"ssid": "mySSID",
									},
								},
								X8021: []map[string]string{
									{
										"key": "value",
									},
								},
								Olpcmesh: []map[string]string{
									{
										"key": "value",
									},
								},
							},
							{
								Name: "test2",
							},
						},
					}}},
			}
			err = def.Apply("foo", config, fs, testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			file, err := fs.ReadFile("/etc/NetworkManager/system-connections/test1.connection")
			Expect(err).ShouldNot(HaveOccurred())
			// Load the written file
			cfg, err := ini.Load(file)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cfg.HasSection("connection")).To(BeTrue())
			Expect(cfg.HasSection("wifi")).To(BeTrue())
			Expect(cfg.Section("connection").HasKey("interface-name")).To(BeTrue())
			Expect(cfg.Section("connection").HasKey("a")).To(BeTrue())
			Expect(cfg.Section("wifi").HasKey("ssid")).To(BeTrue())
			Expect(cfg.Section("connection").Key("interface-name").Value()).To(Equal("eth2"))
			Expect(cfg.Section("connection").Key("a").Value()).To(Equal("b"))
			Expect(cfg.Section("wifi").Key("ssid").Value()).To(Equal("mySSID"))
			// Special sections that are fully renamed
			Expect(cfg.Section("802-1x").Key("key").Value()).To(Equal("value"))
			Expect(cfg.Section("olpc-mesh").Key("key").Value()).To(Equal("value"))

			file, err = fs.ReadFile("/etc/NetworkManager/system-connections/test2.connection")
			Expect(err).ShouldNot(HaveOccurred())
			cfg, err = ini.Load(file)
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})

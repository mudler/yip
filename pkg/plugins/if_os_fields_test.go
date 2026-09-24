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
	"bufio"
	"os"
	"regexp"
	"strings"

	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v5/vfst"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// osRelease returns the requested key from the host's /etc/os-release, which
// is the file sysinfo reads, unquoted the way sysinfo unquotes it.
func osRelease(key string) string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		Skip("no /etc/os-release on this host: " + err.Error())
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		name, value, found := strings.Cut(s.Text(), "=")
		if found && name == key {
			return strings.Trim(value, `"`)
		}
	}
	return ""
}

// readmePatterns returns every regexp the README hands to a given filter key,
// so a documented pattern that RE2 rejects cannot stay documented.
func readmePatterns(key string) []string {
	content, err := os.ReadFile("../../README.md")
	Expect(err).ShouldNot(HaveOccurred())

	// The README writes them as `      only_os: "Ubuntu.*"`.
	line := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(key) + `:\s*"([^"]*)"\s*$`)

	var patterns []string
	for _, m := range line.FindAllStringSubmatch(string(content), -1) {
		patterns = append(patterns, m[1])
	}
	return patterns
}

var _ = Describe("OS conditionals", Label("conditionals"), func() {
	var testConsole consoletests.TestConsole
	var fs *vfst.TestFS
	var cleanup func()

	BeforeEach(func() {
		var err error
		testConsole = consoletests.TestConsole{}
		fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{"/etc/hostname": "boo"})
		Expect(err).Should(BeNil())
	})
	AfterEach(func() {
		testConsole.Reset()
		cleanup()
	})

	// sysinfo puts os-release's PRETTY_NAME in OS.Name and its ID in
	// OS.Vendor, so only_os sees the human-readable string. Anyone who writes
	// a pattern against ID gets a stage that never runs and no error to
	// explain it, which is worth pinning rather than leaving to the reader.
	Describe("only_os", func() {
		It("matches PRETTY_NAME", func() {
			prettyName := osRelease("PRETTY_NAME")
			if prettyName == "" {
				Skip("this host's /etc/os-release has no PRETTY_NAME")
			}

			err := OnlyIfOS(logrus.New(), schema.Stage{
				OnlyIfOs: regexp.QuoteMeta(prettyName),
			}, fs, &testConsole)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("does not match the os-release ID", func() {
			id, prettyName := osRelease("ID"), osRelease("PRETTY_NAME")
			if id == "" || strings.Contains(prettyName, id) {
				Skip("this host's PRETTY_NAME contains its ID, so the two cannot be told apart")
			}

			err := OnlyIfOS(logrus.New(), schema.Stage{
				OnlyIfOs: "^" + regexp.QuoteMeta(id) + "$",
			}, fs, &testConsole)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("doesn't match os name"))
		})
	})

	Describe("only_os_version", func() {
		It("matches VERSION_ID", func() {
			versionID := osRelease("VERSION_ID")
			if versionID == "" {
				Skip("this host's /etc/os-release has no VERSION_ID")
			}

			err := OnlyIfOSVersion(logrus.New(), schema.Stage{
				OnlyIfOsVersion: regexp.QuoteMeta(versionID),
			}, fs, &testConsole)
			Expect(err).ShouldNot(HaveOccurred())
		})
	})

	// Go's regexp is RE2, which has no lookahead. A README example using one
	// does not merely fail to match, it fails to compile, and the stage is
	// skipped with a message about the pattern rather than about the OS.
	Describe("the README's own patterns", func() {
		for _, key := range []string{"only_os", "only_os_version", "only_arch"} {
			key := key
			It("compiles every "+key+" example", func() {
				patterns := readmePatterns(key)
				Expect(patterns).ShouldNot(BeEmpty(), "found no %s example to check", key)

				for _, pattern := range patterns {
					_, err := regexp.Compile(pattern)
					Expect(err).ShouldNot(HaveOccurred(), "README documents %s: %q", key, pattern)
				}
			})
		}
	})
})

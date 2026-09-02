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
	"os/exec"
	"runtime"
	"strings"

	. "github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	consoletests "github.com/mudler/yip/tests/console"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v5/vfst"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Commands", func() {
	Context("parsing yip file", func() {
		testConsole := consoletests.TestConsole{}
		l := logrus.New()
		l.SetOutput(io.Discard)

		BeforeEach(func() {
			testConsole.Reset()
		})
		It("execute commands", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = Commands(l, schema.Stage{
				Commands: []string{"echo foo", "echo bar"},
			}, fs, &testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(testConsole.Commands).Should(Equal([]string{"echo foo", "echo bar"}))
		})
		It("execute templated commands", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{})
			Expect(err).Should(BeNil())
			defer cleanup()
			arch := runtime.GOARCH
			err = Commands(l, schema.Stage{
				Commands: []string{"echo {{.Values.os.architecture}}", "echo bar"},
			}, fs, &testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(testConsole.Commands).Should(Equal([]string{"echo " + arch, "echo bar"}))
		})
		It("hands a shebang command to the kernel, not to sh", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/tmp": &vfst.Dir{Perm: 0o755}})
			Expect(err).Should(BeNil())
			defer cleanup()

			script := "#!/bin/bash\nif [[ foo == foo ]]; then echo yes; fi"
			recorder := &scriptRecorder{}
			err = Commands(l, schema.Stage{Commands: []string{script}}, fs, recorder)
			Expect(err).ShouldNot(HaveOccurred())

			// The console gets a path to run, so the kernel reads the
			// shebang. Passing the text itself makes sh treat it as a
			// comment and run the body under sh.
			Expect(recorder.commands).Should(HaveLen(1))
			Expect(recorder.commands[0]).ShouldNot(ContainSubstring("#!"))
			Expect(recorder.contents[0]).Should(Equal(script))
			Expect(recorder.modes[0].Perm()).Should(Equal(os.FileMode(0o700)))

			// Nothing is left behind once the command has run.
			_, err = os.Stat(recorder.commands[0])
			Expect(os.IsNotExist(err)).Should(BeTrue())
		})
		It("leaves a command that only mentions a shebang alone", func() {
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{"/tmp": &vfst.Dir{Perm: 0o755}})
			Expect(err).Should(BeNil())
			defer cleanup()

			err = Commands(l, schema.Stage{
				Commands: []string{"echo '#!/bin/bash' > /tmp/notascript"},
			}, fs, &testConsole)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(testConsole.Commands).Should(Equal([]string{"echo '#!/bin/bash' > /tmp/notascript"}))
		})
	})
})

// scriptRecorder reads back what a command was asked to run, while the
// file is still on disk.
type scriptRecorder struct {
	commands []string
	contents []string
	modes    []os.FileMode
}

func (r *scriptRecorder) Run(cmd string, opts ...func(*exec.Cmd)) (string, error) {
	r.commands = append(r.commands, cmd)
	info, err := os.Stat(cmd)
	if err != nil {
		return "", err
	}
	r.modes = append(r.modes, info.Mode())
	content, err := os.ReadFile(cmd)
	if err != nil {
		return "", err
	}
	r.contents = append(r.contents, string(content))
	return "", nil
}

func (r *scriptRecorder) Start(cmd *exec.Cmd, opts ...func(*exec.Cmd)) error {
	return nil
}

func (r *scriptRecorder) RunTemplate(st []string, template string) error {
	for _, s := range st {
		if _, err := r.Run(strings.ReplaceAll(template, "%s", s)); err != nil {
			return err
		}
	}
	return nil
}

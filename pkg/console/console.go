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

package console

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/logger"
	"github.com/sirupsen/logrus"
)

type StandardConsole struct {
	logger logger.Interface
}

type StandardConsoleOptions func(*StandardConsole) error

func WithLogger(i logger.Interface) StandardConsoleOptions {
	return func(sc *StandardConsole) error {
		sc.logger = i
		return nil
	}
}

func NewStandardConsole(opts ...StandardConsoleOptions) *StandardConsole {
	c := &StandardConsole{
		logger: logrus.New(),
	}
	for _, o := range opts {
		o(c)
	}
	return c

}

func (s StandardConsole) Run(cmd string, opts ...func(cmd *exec.Cmd)) (string, error) {
	s.logger.Debugf("running command `%s`", cmd)
	c := exec.Command("sh", "-c", cmd)
	for _, o := range opts {
		o(c)
	}
	displayProgress(s.logger, 10*time.Second, fmt.Sprintf("Still running command '%s'", cmd))
	out, err := c.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("failed to run %s: %v", cmd, err)
	}

	return string(out), err
}

func displayProgress(log logger.Interface, tick time.Duration, message string) chan bool {
	ticker := time.NewTicker(tick)
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				ticker.Stop()
				return
			case <-ticker.C:
				log.Info(message)
			}
		}
	}()

	return done
}

func (s StandardConsole) Start(cmd *exec.Cmd, opts ...func(cmd *exec.Cmd)) error {
	s.logger.Debugf("running command `%s`", cmd)
	for _, o := range opts {
		o(cmd)
	}
	return cmd.Run()
}

func (s StandardConsole) RunTemplate(st []string, template string) error {
	var errs error

	for _, svc := range st {
		out, err := s.Run(fmt.Sprintf(template, svc))
		if err != nil {
			s.logger.Error(out)
			s.logger.Error(err.Error())
			errs = multierror.Append(errs, err)
			continue
		}
	}
	return errs
}

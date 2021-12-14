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

package console

import (
	"fmt"
	"os/exec"

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
	out, err := c.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("failed to run %s: %v", cmd, err)
	}

	return string(out), err
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

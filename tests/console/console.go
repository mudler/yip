package consoletests

import (
	"fmt"
	"os/exec"

	"github.com/apex/log"
	"github.com/hashicorp/go-multierror"
)

type TestConsole struct {
	Commands []string
}

func (s *TestConsole) Run(cmd string, opts ...func(*exec.Cmd)) (string, error) {
	c := &exec.Cmd{}
	for _, o := range opts {
		o(c)
	}
	s.Commands = append(s.Commands, cmd)
	s.Commands = append(s.Commands, c.Args...)

	return "", nil
}

func (s *TestConsole) Reset() {
	s.Commands = []string{}
}
func (s *TestConsole) Start(cmd *exec.Cmd, opts ...func(*exec.Cmd)) error {
	for _, o := range opts {
		o(cmd)
	}
	s.Commands = append(s.Commands, cmd.Args...)
	return nil
}

func (s *TestConsole) RunTemplate(st []string, template string) error {
	var errs error

	for _, svc := range st {
		out, err := s.Run(fmt.Sprintf(template, svc))
		if err != nil {
			log.Error(out)
			log.Error(err.Error())
			errs = multierror.Append(errs, err)
			continue
		}
	}
	return errs
}

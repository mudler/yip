package plugins

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/schema"
	log "github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs"
)

func setUser(username, password string, console Console) error {
	if password == "" {
		return nil
	}
	chpasswd := "chpasswd"
	out, err := console.Run(chpasswd, func(c *exec.Cmd) {
		c.Path = chpasswd
		if strings.HasPrefix(password, "$") {
			c.Args = append(c.Args, "-e")
		}
		c.Stdin = strings.NewReader(fmt.Sprintf("%s:%s", username, password))
	})
	if err != nil {
		log.Info(fmt.Sprintf("Command output: %s", string(out)))
		log.Error(err.Error())
	}
	return err
}

func User(s schema.Stage, fs vfs.FS, console Console) error {
	var errs error

	for u, p := range s.Users {
		if err := setUser(u, p, console); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

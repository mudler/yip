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

func createUser(u schema.User, console Console) error {
	var reserr error
	args := []string{}

	if u.GECOS != "" {
		args = append(args, "-g", fmt.Sprintf("%q", u.GECOS))
	}

	if u.Homedir != "" {
		args = append(args, "-h", u.Homedir)
	}

	if u.NoCreateHome {
		args = append(args, "-H")
	}

	if u.PrimaryGroup != "" {
		args = append(args, "-G", u.PrimaryGroup)
	}

	if u.System {
		args = append(args, "-S")
	}

	if u.Shell != "" {
		args = append(args, "-s", u.Shell)
	}

	args = append(args, "-D")
	args = append(args, u.Name)

	adduserCmd := []string{"adduser"}
	output, err := console.Run(strings.Join(append(adduserCmd, args...), " "))
	if err != nil {
		reserr = multierror.Append(reserr, err)
		log.Printf("Command 'useradd %s' failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	if len(u.Groups) > 0 {
		for _, group := range u.Groups {
			args := []string{u.Name, group}
			output, err := console.Run(strings.Join(append(adduserCmd, args...), " "))
			if err != nil {
				reserr = multierror.Append(reserr, err)

				log.Printf("Command 'adduser %s' failed: %v\n%s", strings.Join(args, " "), err, output)
			}
		}
	}
	if u.PasswordHash != "" {
		err := setUserPassword(u.Name, u.PasswordHash, console)
		if err != nil {
			reserr = multierror.Append(reserr, err)
			log.Printf("Error setting password for %s: %v\n", u.Name, err)
		}
	}
	return reserr
}

func setUserPassword(username, password string, console Console) error {
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
		p.Name = u
		if !p.Exists() {
			if err := createUser(p, console); err != nil {
				errs = multierror.Append(errs, err)
			}
		} else {
			if err := setUserPassword(u, p.PasswordHash, console); err != nil {
				errs = multierror.Append(errs, err)
			}
		}
	}
	return errs
}

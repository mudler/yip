package plugins

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs"
)

func SystemdFirstboot(s schema.Stage, fs vfs.FS, console Console) error {
	var errs error

	args := []string{}

	for k, v := range s.SystemdFirstBoot {
		args = append(args, fmt.Sprintf("--%s=%s", strings.ToLower(k), v))
	}

	if err := console.RunTemplate(args, "systemd-firstboot %s"); err != nil {
		errs = multierror.Append(errs, err)
	}

	return errs
}

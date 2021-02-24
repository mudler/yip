package plugins

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/schema"
	log "github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs"
)

func run(s []string, template string, console Console) error {
	var errs error

	for _, svc := range s {
		out, err := console.Run(fmt.Sprintf(template, svc))
		if err != nil {
			log.Error(out)
			log.Error(err.Error())
			errs = multierror.Append(errs, err)
			continue
		}
	}
	return errs
}

func Systemctl(s schema.Stage, fs vfs.FS, console Console) error {
	var errs error

	if err := run(s.Systemctl.Enable, "systemctl enable %s", console); err != nil {
		errs = multierror.Append(errs, err)
	}
	if err := run(s.Systemctl.Disable, "systemctl disable %s", console); err != nil {
		errs = multierror.Append(errs, err)
	}
	if err := run(s.Systemctl.Mask, "systemctl mask %s", console); err != nil {
		errs = multierror.Append(errs, err)
	}
	if err := run(s.Systemctl.Start, "systemctl start %s", console); err != nil {
		errs = multierror.Append(errs, err)
	}
	return errs
}

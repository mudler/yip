package plugins

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/schema"
	log "github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs"
)

func Commands(s schema.Stage, fs vfs.FS, console Console) error {
	var errs error
	for _, cmd := range s.Commands {
		out, err := console.Run(templateSysData(cmd))
		if err != nil {
			log.Error(out, ": ",  err.Error())
			errs = multierror.Append(errs, err)
			continue
		}
		log.Info(fmt.Sprintf("Command output: %s", string(out)))
	}
	return errs
}

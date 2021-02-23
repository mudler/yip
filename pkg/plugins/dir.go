package plugins

import (
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/schema"
	log "github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs"
)

func EnsureDirectories(s schema.Stage, fs vfs.FS, console Console) error {
	var errs error
	for _, dir := range s.Directories {
		if err := writeDirectory(dir, fs); err != nil {
			log.Error(err.Error())
			errs = multierror.Append(errs, err)
			continue
		}
	}
	return errs
}

func writeDirectory(dir schema.Directory, fs vfs.FS) error {
	log.Debug("Creating directory ", dir.Path)
	err := fs.Mkdir(dir.Path, os.FileMode(dir.Permissions))
	if err != nil {
		return err
	}

	return fs.Chown(dir.Path, dir.Owner, dir.Group)
}

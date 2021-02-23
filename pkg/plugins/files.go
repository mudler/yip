package plugins

import (
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/schema"
	log "github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs"
)

func EnsureFiles(s schema.Stage, fs vfs.FS, console Console) error {
	var errs error
	for _, file := range s.Files {
		if err := writeFile(file, fs); err != nil {
			log.Error(err.Error())
			errs = multierror.Append(errs, err)
			continue
		}
	}
	return errs
}

func writeFile(file schema.File, fs vfs.FS) error {
	log.Debug("Creating file ", file.Path)
	fsfile, err := fs.Create(file.Path)
	if err != nil {
		return err
	}

	_, err = fsfile.WriteString(templateSysData(file.Content))
	if err != nil {
		return err

	}
	err = fs.Chmod(file.Path, os.FileMode(file.Permissions))
	if err != nil {
		return err

	}
	return fs.Chown(file.Path, file.Owner, file.Group)
}

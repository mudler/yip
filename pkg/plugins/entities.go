package plugins

import (
	"github.com/apex/log"
	"github.com/hashicorp/go-multierror"
	entities "github.com/mudler/entities/pkg/entities"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs"
)

func Entities(s schema.Stage, fs vfs.FS, console Console) error {
	var errs error
	if len(s.EnsureEntities) > 0 {
		if err := ensureEntities(s); err != nil {
			log.Error(err.Error())
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

func DeleteEntities(s schema.Stage, fs vfs.FS, console Console) error {
	var errs error
	if len(s.DeleteEntities) > 0 {
		if err := deleteEntities(s); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

func deleteEntities(s schema.Stage) error {
	var errs error
	entityParser := entities.Parser{}
	for _, e := range s.DeleteEntities {
		decodedE, err := entityParser.ReadEntityFromBytes([]byte(templateSysData(e.Entity)))
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
		err = decodedE.Delete(e.Path)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
	}
	return errs
}

func ensureEntities(s schema.Stage) error {
	var errs error
	entityParser := entities.Parser{}
	for _, e := range s.EnsureEntities {
		decodedE, err := entityParser.ReadEntityFromBytes([]byte(templateSysData(e.Entity)))
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
		err = decodedE.Apply(e.Path)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
	}
	return errs
}

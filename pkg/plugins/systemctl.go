package plugins

import (
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs/v4"
	"path/filepath"
)

const (
	ErrorEmptyOverrideService = "Skipping empty override service"
	ErrorEmptyOverrideContent = "Empty override content for %s"
	DefaultOverrideName       = "override-yip.conf"
	DefaultOverrideDir        = "/etc/systemd/system/%s.d"
	DefaultServiceExt         = ".service"
	DefaultOverrideExt        = ".conf"
	EmptyString               = ""
)

func Systemctl(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	var errs error

	if err := console.RunTemplate(s.Systemctl.Enable, "systemctl enable %s"); err != nil {
		errs = multierror.Append(errs, err)
	}
	if err := console.RunTemplate(s.Systemctl.Disable, "systemctl disable %s"); err != nil {
		errs = multierror.Append(errs, err)
	}
	if err := console.RunTemplate(s.Systemctl.Mask, "systemctl mask %s"); err != nil {
		errs = multierror.Append(errs, err)
	}
	if err := console.RunTemplate(s.Systemctl.Start, "systemctl start %s"); err != nil {
		errs = multierror.Append(errs, err)
	}
	for _, override := range s.Systemctl.Overrides {
		// Skip empty overrides or empty content
		if override.Service == EmptyString {
			l.Warnf(ErrorEmptyOverrideService)
			continue
		}
		if override.Content == EmptyString {
			l.Warnf(ErrorEmptyOverrideContent, override.Service)
			continue
		}
		// Override name is optional, default to override-yip.conf
		if override.Name == EmptyString {
			override.Name = DefaultOverrideName
		}
		// Ensure the extension is .conf
		if filepath.Ext(override.Name) != DefaultOverrideExt {
			override.Name = override.Name + DefaultOverrideExt
		}
		// Ensure the service has a .service extension
		if filepath.Ext(override.Service) != DefaultServiceExt {
			override.Service = override.Service + DefaultServiceExt
		}
		// Create the override directory
		overrideDir := fmt.Sprintf(DefaultOverrideDir, override.Service)
		err := vfs.MkdirAll(fs, overrideDir, 0755)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
		// Write the override file content
		err = fs.WriteFile(filepath.Join(overrideDir, override.Name), []byte(override.Content), 0644)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

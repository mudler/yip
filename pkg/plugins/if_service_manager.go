package plugins

import (
	"fmt"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs/v4"
	"strings"
)

const SkipOnlyServiceManager = "service manager doesn't match %s"
const SkipBothServices = "both systemd and openrc are available, cant filter"
const SkipNotSupportedServiceManager = "service manager %s is not supported"

// IfServiceManager checks if the current service manager matches the one specified in the stage
// Only runs if the service manager matches the specified service manager
func IfServiceManager(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	if s.OnlyIfServiceManager != "" {
		if strings.ToLower(s.OnlyIfServiceManager) != "systemd" && strings.ToLower(s.OnlyIfServiceManager) != "openrc" {
			return fmt.Errorf(fmt.Sprintf(SkipNotSupportedServiceManager, s.OnlyIfServiceManager))
		}

		var isSystemd, isOpenRC bool

		for _, c := range []string{"/sbin/systemctl", "/usr/bin/systemctl", "/usr/sbin/systemctl", "/usr/bin/systemctl"} {
			if _, ok := fs.Stat(c); ok == nil {
				isSystemd = true
				break
			}
		}

		for _, c := range []string{"/sbin/openrc", "/usr/bin/openrc", "/usr/sbin/openrc", "/usr/bin/openrc"} {
			if _, ok := fs.Stat(c); ok == nil {
				isOpenRC = true
				break
			}
		}

		if isSystemd && isOpenRC {
			return fmt.Errorf(SkipBothServices)
		}

		if strings.ToLower(s.OnlyIfServiceManager) == "systemd" && isSystemd {
			return nil
		}
		if strings.ToLower(s.OnlyIfServiceManager) == "openrc" && isOpenRC {
			return nil
		}

		return fmt.Errorf(fmt.Sprintf(SkipOnlyServiceManager, s.OnlyIfServiceManager))

	}
	return nil
}

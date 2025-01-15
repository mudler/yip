package plugins

import (
	"fmt"
	"regexp"

	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs/v4"
)

// OnlyIfOS checks if the OS matches the if statement and runs it if so
func OnlyIfOS(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	l.Info("Running OnlyIfOS")
	if s.OnlyIfOs != "" {
		compile, err := regexp.Compile(s.OnlyIfOs)
		if err != nil {
			l.Debugf("Skipping stage (OnlyIfOs regex compile (%s) statement error: %w)", s.OnlyIfOs, err)
			return err
		}

		// Get the OS name from the system
		system.GetSysInfo()
		if compile.MatchString(system.OS.Name) {
			return nil
		}
		return fmt.Errorf("skipping stage (OnlyIfOs regex (%s) doesn't match os name '%s')", s.OnlyIfOs, system.OS.Name)
	}
	return nil
}

// OnlyIfOSVersion checks if the OS VERSION matches the if statement and runs it if so
func OnlyIfOSVersion(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	l.Info("Running OnlyIfOSVersion")
	if s.OnlyIfOs != "" {
		compile, err := regexp.Compile(s.OnlyIfOsVersion)
		if err != nil {
			l.Debugf("Skipping stage (OnlyIfOsVersion regex compile (%s) statement error: %w)", s.OnlyIfOsVersion, err)
			return err
		}

		// Get the OS version from the system
		system.GetSysInfo()
		if compile.MatchString(system.OS.Version) {
			return nil
		}
		return fmt.Errorf("skipping stage (OnlyIfOsVersion regex (%s) doesn't match os version '%s')", s.OnlyIfOs, system.OS.Version)
	}
	return nil
}

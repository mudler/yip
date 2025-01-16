package plugins

import (
	"fmt"
	"regexp"

	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs/v4"
)

const SkipOnlyOs = "OnlyIfOs regex (%s)"
const SkipOnlyOsVersion = "OnlyIfOsVersion regex (%s)"
const RunOnlyOs = "running stage (OnlyIfOs regex (%s)"
const RunOnlyOsVersion = "running stage (OnlyIfOsVersion regex (%s)"

// OnlyIfOS checks if the OS matches the if statement and runs it if so
func OnlyIfOS(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	if s.OnlyIfOs != "" {
		compile, err := regexp.Compile(s.OnlyIfOs)
		if err != nil {
			l.Debugf("%s compile statement error: %w", fmt.Sprintf(SkipOnlyOs, s.OnlyIfOs), err)
			return err
		}

		// Get the OS name from the system
		system.GetSysInfo()
		if system.OS.Name == "" {
			return fmt.Errorf("%s as system os name is empty", fmt.Sprintf(SkipOnlyOs, s.OnlyIfOs))
		}
		if compile.MatchString(system.OS.Name) {
			l.Debugf("%s matches os name '%s", fmt.Sprintf(RunOnlyOs, s.OnlyIfOs), system.OS.Name)
			return nil
		}
		return fmt.Errorf("%s doesn't match os name %s", fmt.Sprintf(SkipOnlyOs, s.OnlyIfOs), system.OS.Name)
	}
	return nil
}

// OnlyIfOSVersion checks if the OS VERSION matches the if statement and runs it if so
func OnlyIfOSVersion(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	if s.OnlyIfOsVersion != "" {
		compile, err := regexp.Compile(s.OnlyIfOsVersion)
		if err != nil {
			l.Debugf("%s compile statement error: %w", fmt.Sprintf(SkipOnlyOsVersion, s.OnlyIfOsVersion), err)
			return err
		}

		// Get the OS version from the system
		system.GetSysInfo()
		if system.OS.Version == "" {
			return fmt.Errorf("%s as system version is empty", fmt.Sprintf(SkipOnlyOsVersion, s.OnlyIfOsVersion))
		}
		if compile.MatchString(system.OS.Version) {
			l.Debugf("%s matches os version '%s'", fmt.Sprintf(RunOnlyOsVersion, s.OnlyIfOsVersion), system.OS.Version)
			return nil
		}
		return fmt.Errorf("%s doesn't match os version %s", fmt.Sprintf(SkipOnlyOsVersion, s.OnlyIfOsVersion), system.OS.Version)
	}
	return nil
}

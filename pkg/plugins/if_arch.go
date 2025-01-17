package plugins

import (
	"fmt"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs/v4"
	"regexp"
	"runtime"
)

const SkipOnlyArch = "arch %s doesn't match %s"

// IfArch checks if the current architecture matches the one specified in the stage
// Only runs if the regex matches the current architecture
func IfArch(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	if s.OnlyIfArch != "" {
		re, err := regexp.Compile(s.OnlyIfArch)
		if err != nil {
			return fmt.Errorf("failed to compile regex %s: %w", s.OnlyIfArch, err)
		}
		if !re.MatchString(runtime.GOARCH) {
			return fmt.Errorf(fmt.Sprintf(SkipOnlyArch, runtime.GOARCH, s.OnlyIfArch))
		}
	}
	return nil
}

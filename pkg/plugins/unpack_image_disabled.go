//go:build nounpack

package plugins

import (
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs/v4"
)

func UnpackImage(l logger.Interface, s schema.Stage, _ vfs.FS, _ Console) error {
	if len(s.UnpackImages) == 0 {
		return nil
	}
	l.Warn("Unpack image plugin is disabled at build time")
	return nil
}

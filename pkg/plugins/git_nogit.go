//go:build nogit && !gitbinary

package plugins

import (
	"fmt"

	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs/v4"
)

func Git(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	if s.Git.URL == "" {
		return nil
	}
	return fmt.Errorf("git plugin not available in nogit build")
}

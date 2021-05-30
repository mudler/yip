package plugins

import (
	"fmt"

	"github.com/mudler/yip/pkg/schema"
	log "github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs"
)

func IfConditional(s schema.Stage, fs vfs.FS, console Console) error {
	if len(s.If) > 0 {
		out, err := console.Run(templateSysData(s.If))
		if err != nil {
			return fmt.Errorf("Skipping stage (if statement didn't passed)")
		}
		log.Debugf("If statement result %s", out)
	}
	return nil
}

package plugins

import (
	"fmt"

	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs/v4"
)

func IfFiles(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	if len(s.IfFiles) > 0 {
		for check, files := range s.IfFiles {
			switch check {
			// Check that all files exist
			case schema.IfCheckAll:
				if len(files) == 0 {
					return nil
				}
				for _, file := range files {
					_, err := fs.Stat(file)
					// if one file does not exist, skip the stage
					if err != nil {
						return fmt.Errorf("skipping stage, file %s missing", file)
					}
				}
			// Check that at least one file exists
			case schema.IfCheckAny:
				if len(files) == 0 {
					return nil
				}
				found := false
				for _, file := range files {
					_, err := fs.Stat(file)
					if err == nil {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("skipping stage, none of the files exist)")
				}

			// Check that no files exist
			case schema.IfCheckNone:
				if len(files) == 0 {
					return nil
				}
				for _, file := range files {
					_, err := fs.Stat(file)
					// if one file exists, skip the stage
					if err == nil {
						return fmt.Errorf("skipping stage, file %s exists", file)
					}
				}

			default:
				return fmt.Errorf("unknown if_files check type: %s", check)
			}
		}
	}
	return nil
}

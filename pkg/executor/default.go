// Copyright © 2020 Ettore Di Giacinto <mudler@gentoo.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package executor

import (
	"fmt"
	"os"

	"github.com/ionrock/procs"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs"
)

// DefaultExecutor is the default yip Executor.
// It simply creates file and executes command for a linux executor
type DefaultExecutor struct{}

// Apply applies a yip Config file by creating files and running commands defined.
func (e *DefaultExecutor) Apply(stage string, s schema.YipConfig, fs vfs.FS) error {
	currentStages, _ := s.Stages[stage]

	for _, stage := range currentStages {
		for _, file := range stage.Files {
			fmt.Println("Creating file", file.Path)
			fsfile, err := fs.Create(file.Path)
			if err != nil {
				fmt.Println(err)
				continue
			}

			_, err = fsfile.WriteString(file.Content)
			if err != nil {
				fmt.Println(err)
				continue
			}
			err = fs.Chmod(file.Path, os.FileMode(file.Permissions))
			if err != nil {
				fmt.Println(err)
				continue
			}
			err = fs.Chown(file.Path, file.Owner, file.Group)
			if err != nil {
				fmt.Println(err)
				continue
			}
		}

		for _, cmd := range stage.Commands {
			fmt.Println("Running", cmd)

			p := procs.NewProcess(cmd)
			err := p.Run()
			if err != nil {
				fmt.Println(err)
				continue
			}
			out, _ := p.Output()
			fmt.Println(string(out))
		}
	}
	return nil
}

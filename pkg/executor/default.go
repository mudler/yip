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

	resolvconf "github.com/moby/libnetwork/resolvconf"
	entities "github.com/mudler/entities/pkg/entities"

	"github.com/hashicorp/go-multierror"
	"github.com/ionrock/procs"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs"
)

// DefaultExecutor is the default yip Executor.
// It simply creates file and executes command for a linux executor
type DefaultExecutor struct{}

func (e *DefaultExecutor) applyDNS(s schema.YipConfig) error {
	path := s.Dns.Path
	if path == "" {
		path = "/etc/resolv.conf"
	}
	_, err := resolvconf.Build(path, s.Dns.Nameservers, s.Dns.DnsSearch, s.Dns.DnsOptions)
	return err
}

func (e *DefaultExecutor) ensureEntities(s schema.YipConfig) error {
	var errs error
	entityParser := entities.Parser{}
	for _, e := range s.EnsureEntities {
		decodedE, err := entityParser.ReadEntityFromBytes([]byte(e.Entity))
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
		err = decodedE.Apply(e.Path)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
	}
	return errs
}

func (e *DefaultExecutor) deleteEntities(s schema.YipConfig) error {
	var errs error
	entityParser := entities.Parser{}
	for _, e := range s.DeleteEntities {
		decodedE, err := entityParser.ReadEntityFromBytes([]byte(e.Entity))
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
		err = decodedE.Delete(e.Path)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
	}
	return errs
}

func (e *DefaultExecutor) writeFile(file schema.File, fs vfs.FS) error {
	fmt.Println("Creating file", file.Path)
	fsfile, err := fs.Create(file.Path)
	if err != nil {
		return err
	}

	_, err = fsfile.WriteString(file.Content)
	if err != nil {
		return err

	}
	err = fs.Chmod(file.Path, os.FileMode(file.Permissions))
	if err != nil {
		return err

	}
	return fs.Chown(file.Path, file.Owner, file.Group)
}

func (e *DefaultExecutor) runProc(cmd string) (string, error) {
	fmt.Println("Running", cmd)

	p := procs.NewProcess(cmd)
	err := p.Run()
	if err != nil {
		return "", err
	}
	out, err := p.Output()
	return string(out), err
}

// Apply applies a yip Config file by creating files and running commands defined.
func (e *DefaultExecutor) Apply(stage string, s schema.YipConfig, fs vfs.FS) error {
	currentStages, _ := s.Stages[stage]
	var errs error

	if len(s.Dns.Nameservers) != 0 {
		if err := e.applyDNS(s); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	if len(s.EnsureEntities) > 0 {
		if err := e.ensureEntities(s); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	for _, stage := range currentStages {
		for _, file := range stage.Files {
			if err := e.writeFile(file, fs); err != nil {
				fmt.Println(err)
				errs = multierror.Append(errs, err)
				continue
			}
		}

		for _, cmd := range stage.Commands {
			out, err := e.runProc(cmd)
			if err != nil {
				fmt.Println(err)
				errs = multierror.Append(errs, err)
				continue
			}
			fmt.Println(string(out))
		}
	}

	if len(s.DeleteEntities) > 0 {
		if err := e.deleteEntities(s); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

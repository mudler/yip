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
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	"github.com/mudler/yip/pkg/utils"
	"github.com/twpayne/go-vfs"
)

// DefaultExecutor is the default yip Executor.
// It simply creates file and executes command for a linux executor
type DefaultExecutor struct {
	plugins      []Plugin
	conditionals []Plugin
}

func (e *DefaultExecutor) Plugins(p []Plugin) {
	e.plugins = p
}

func (e *DefaultExecutor) Conditionals(p []Plugin) {
	e.conditionals = p
}

func (e *DefaultExecutor) walkDir(stage, dir string, fs vfs.FS, console plugins.Console) error {
	var errs error

	err := vfs.Walk(fs, dir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if path == dir {
				return nil
			}
			// Process only files
			if info.IsDir() {
				return nil
			}
			ext := filepath.Ext(path)
			if ext != ".yaml" && ext != ".yml" {
				return nil
			}
			config, err := schema.LoadFromFile(path, fs)
			if err != nil {
				errs = multierror.Append(errs, err)
				return nil
			}
			log.Infof("Executing %s", path)
			if err = e.Apply(stage, *config, vfs.OSFS, console); err != nil {
				errs = multierror.Append(errs, err)
				return nil
			}

			return nil
		})
	if err != nil {
		errs = multierror.Append(errs, err)
	}
	return errs
}

func (e *DefaultExecutor) runRemoteFile(stage, uri string, fs vfs.FS, console plugins.Console) error {
	config, err := schema.LoadFromUrl(uri)
	if err != nil {
		return err
	}

	if err = e.Apply(stage, *config, fs, console); err != nil {
		return err
	}

	return nil
}

func (e *DefaultExecutor) runFile(stage, uri string, fs vfs.FS, console plugins.Console) error {
	config, err := schema.LoadFromFile(uri, fs)
	if err != nil {
		return err
	}

	if err = e.Apply(stage, *config, fs, console); err != nil {
		return err
	}

	return nil
}

func (e *DefaultExecutor) runStage(stage, uri string, fs vfs.FS, console plugins.Console) (err error) {
	f, err := fs.Stat(uri)
	if err == nil && f.IsDir() {
		// Load yamls in a directory
		err = e.walkDir(stage, uri, fs, console)
	} else if err == nil {
		err = e.runFile(stage, uri, fs, console)
	} else if utils.IsUrl(uri) {
		err = e.runRemoteFile(stage, uri, fs, console)
	}
	return
}

// Run takes a list of URI to run yipfiles from. URI can be also a dir or a local path, as well as a remote
func (e *DefaultExecutor) Run(stage string, fs vfs.FS, console plugins.Console, args ...string) error {
	var errs error

	for _, source := range args {
		if err := e.runStage(stage, source, fs, console); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

// Apply applies a yip Config file by creating files and running commands defined.
func (e *DefaultExecutor) Apply(stageName string, s schema.YipConfig, fs vfs.FS, console plugins.Console) error {

	currentStages, _ := s.Stages[stageName]
	var errs error

	log.WithFields(log.Fields{
		"name":   s.Name,
		"stages": len(currentStages),
		"stage":  stageName,
	}).Info("Executing yip file")
STAGES:
	for _, stage := range currentStages {

		for _, p := range e.conditionals {
			if err := p(stage, fs, console); err != nil {
				log.WithFields(log.Fields{
					"name":  s.Name,
					"stage": stageName,
				}).Warning(err.Error())
				continue STAGES
			}
		}

		log.WithFields(log.Fields{
			"commands":        len(stage.Commands),
			"entities":        len(stage.EnsureEntities),
			"nameserver":      len(stage.Dns.Nameservers),
			"files":           len(stage.Files),
			"delete_entities": len(stage.DeleteEntities),
			"step":            stage.Name,
		}).Info(fmt.Sprintf("Processing stage step '%s'", stage.Name))

		for _, p := range e.plugins {
			if err := p(stage, fs, console); err != nil {
				log.Error(err.Error())
				errs = multierror.Append(errs, err)
			}
		}
	}

	log.WithFields(log.Fields{
		"success": errs == nil,
		"stages":  len(currentStages),
		"stage":   stageName,
	}).Info("Finished yip file execution")
	return errs
}

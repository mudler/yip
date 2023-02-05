//   Copyright 2020 Ettore Di Giacinto <mudler@mocaccino.org>
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	"github.com/mudler/yip/pkg/utils"
	"github.com/spectrocloud-labs/herd"
	"github.com/twpayne/go-vfs"
)

// DefaultExecutor is the default yip Executor.
// It simply creates file and executes command for a linux executor
type DefaultExecutor struct {
	g            *herd.Graph
	plugins      []Plugin
	conditionals []Plugin
	modifier     schema.Modifier
	logger       logger.Interface
}

func (e *DefaultExecutor) Plugins(p []Plugin) {
	e.plugins = p
}

func (e *DefaultExecutor) Conditionals(p []Plugin) {
	e.conditionals = p
}

func (e *DefaultExecutor) Modifier(m schema.Modifier) {
	e.modifier = m
}

type op struct {
	fn      func(context.Context) error
	deps    []string
	options []herd.OpOption
	name    string
}

func (e *DefaultExecutor) applyStage(stage schema.Stage, fs vfs.FS, console plugins.Console) error {
	var errs error
	for _, p := range e.conditionals {
		if err := p(e.logger, stage, fs, console); err != nil {
			e.logger.Warnf("Skip '%s' stage name: %s\n",
				err.Error(), stage.Name)
			return nil
		}
	}

	e.logger.Infof(
		"Processing stage step '%s'. ( commands: %d, files: %d, ... )\n",
		stage.Name,
		len(stage.Commands),
		len(stage.Files))

	b, _ := json.Marshal(stage)
	e.logger.Debugf("Stage: %s", string(b))

	for _, p := range e.plugins {
		if err := p(e.logger, stage, fs, console); err != nil {
			e.logger.Error(err.Error())
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

func (e *DefaultExecutor) genOpFromSchema(file, stage string, config schema.YipConfig, fs vfs.FS, console plugins.Console) []*op {
	results := []*op{}

	currentStages := config.Stages[stage]

	prev := ""
	for i, st := range currentStages {
		name := st.Name
		if name == "" {
			name = fmt.Sprint(i)
		}
		rootname := file
		if config.Name != "" {
			rootname = config.Name
		}

		opName := fmt.Sprintf("%s-%s-%s", rootname, stage, name)

		o := &op{
			fn: func(ctx context.Context) error {
				e.logger.Infof("Executing %s", file)
				return e.applyStage(st, fs, console)
			},
			name: opName,
		}

		for _, d := range st.Depends {
			o.deps = append(o.deps, d.Name)
		}

		if i != 0 {
			o.deps = append(o.deps, prev)
		}

		results = append(results, o)

		prev = opName
	}

	return results
}

func (e *DefaultExecutor) dirOps(stage, dir string, fs vfs.FS, console plugins.Console) ([]*op, error) {
	results := []*op{}
	prev := []string{}
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

			config, err := schema.Load(path, fs, schema.FromFile, e.modifier)
			if err != nil {
				return err

			}
			ops := e.genOpFromSchema(path, stage, *config, fs, console)
			if len(prev) > 0 {
				fmt.Println("APPENDING")
				for _, o := range ops {
					o.deps = append(o.deps, prev...)
				}
			}
			prev = []string{}
			for _, o := range ops {
				fmt.Println("APPENDING ops", o.name)

				prev = append(prev, o.name)
			}
			results = append(results, ops...)
			return nil
		})
	return results, err
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

			if err = e.run(stage, path, fs, console, schema.FromFile, e.modifier); err != nil {
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

func (e *DefaultExecutor) run(stage, uri string, fs vfs.FS, console plugins.Console, l schema.Loader, m schema.Modifier) error {
	config, err := schema.Load(uri, fs, l, m)
	if err != nil {
		return err
	}

	e.logger.Infof("Executing %s", uri)
	if err = e.Apply(stage, *config, fs, console); err != nil {
		return err
	}

	return nil
}

func (e *DefaultExecutor) runStage(stage, uri string, fs vfs.FS, console plugins.Console) (err error) {
	f, err := fs.Stat(uri)

	switch {
	case err == nil && f.IsDir():

		ops, err := e.dirOps(stage, uri, fs, console)
		if err != nil {
			return err
		}
		for _, o := range ops {
			e.g.Add(o.name, herd.WithCallback(o.fn), herd.WithDeps(o.deps...))
		}
		fmt.Println(e.g.Analyze())
		return e.g.Run(context.Background())
	case err == nil:
		err = e.run(stage, uri, fs, console, schema.FromFile, e.modifier)
	case utils.IsUrl(uri):
		err = e.run(stage, uri, fs, console, schema.FromUrl, e.modifier)
	default:

		err = e.run(stage, uri, fs, console, nil, e.modifier)
	}

	return
}

// Run takes a list of URI to run yipfiles from. URI can be also a dir or a local path, as well as a remote
func (e *DefaultExecutor) Run(stage string, fs vfs.FS, console plugins.Console, args ...string) error {
	var errs error
	e.logger.Infof("Running stage: %s\n", stage)
	for _, source := range args {
		if err := e.runStage(stage, source, fs, console); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	e.logger.Infof("Done executing stage '%s'\n", stage)
	return errs
}

// Apply applies a yip Config file by creating files and running commands defined.
func (e *DefaultExecutor) Apply(stageName string, s schema.YipConfig, fs vfs.FS, console plugins.Console) error {
	currentStages := s.Stages[stageName]
	if len(currentStages) == 0 {
		e.logger.Debugf("No commands to run for %s %s\n", stageName, s.Name)
		return nil
	}

	e.logger.Infof("Applying '%s' for stage '%s'. Total stages: %d\n", s.Name, stageName, len(currentStages))

	var errs error
STAGES:
	for _, stage := range currentStages {
		for _, p := range e.conditionals {
			if err := p(e.logger, stage, fs, console); err != nil {
				e.logger.Warnf("Error '%s' in stage name: %s stage: %s\n",
					err.Error(), s.Name, stageName)
				continue STAGES
			}
		}

		e.logger.Infof(
			"Processing stage step '%s'. ( commands: %d, files: %d, ... )\n",
			stage.Name,
			len(stage.Commands),
			len(stage.Files))

		b, _ := json.Marshal(stage)
		e.logger.Debugf("Stage: %s", string(b))

		for _, p := range e.plugins {
			if err := p(e.logger, stage, fs, console); err != nil {
				e.logger.Error(err.Error())
				errs = multierror.Append(errs, err)
			}
		}
	}

	e.logger.Infof(
		"Stage '%s'. Defined stages: %d. Errors: %t\n",
		stageName,
		len(currentStages),
		errs != nil,
	)

	return errs
}

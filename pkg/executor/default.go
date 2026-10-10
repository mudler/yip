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
	"github.com/sanity-io/litter"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/plugins"
	"github.com/mudler/yip/pkg/schema"
	"github.com/mudler/yip/pkg/utils"
	"github.com/spectrocloud-labs/herd"
	"github.com/twpayne/go-vfs/v5"
)

// DefaultExecutor is the default yip Executor.
// It simply creates file and executes command for a linux executor
type DefaultExecutor struct {
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
	after   []string
	options []herd.OpOption
	name    string
}

type opList []*op

func (l opList) uniqueNames() {
	names := map[string]int{}

	for _, op := range l {
		if names[op.name] > 0 {
			op.name = fmt.Sprintf("%s.%d", op.name, names[op.name])
			names[op.name] = names[op.name] + 1
		} else {
			names[op.name] = 1
		}
	}
}

func (e *DefaultExecutor) applyStage(config schema.YipConfig, stageName string, stage schema.Stage, fs vfs.FS, console plugins.Console) error {
	var errs error
	for _, p := range e.conditionals {
		if err := p(e.logger, stage, fs, console); err != nil {
			e.logger.Warnf("(conditional) Skip '%s' stage name: %s",
				err.Error(), stageName)
			return nil
		}
	}
	e.logger.Infof(
		"Processing stage step '%s'. ( commands: %d, files: %d, ... )",
		stageName,
		len(stage.Commands),
		len(stage.Files))

	litter.Config.HideZeroValues = true
	e.logger.Debugf("Stage: %s", litter.Sdump(stage))

	for _, p := range e.plugins {
		ctx, cancel := context.WithCancel(context.Background())
		go stillAlive(ctx, e.logger, 10*time.Second, fmt.Sprintf("Still running stage '%s'", stageName))
		if err := p(e.logger, stage, fs, console); err != nil {
			e.logger.Errorf("Error on file %s on stage %s: %s", config.Source, stage.Name, err)
			errs = multierror.Append(errs, err)
		}
		cancel()
	}
	return errs
}

func stillAlive(ctx context.Context, log logger.Interface, tick time.Duration, message string) {
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Info(message)
		}
	}
}

func checkDuplicates(stages []schema.Stage) bool {
	stageNames := map[string]bool{}
	for _, st := range stages {
		if _, ok := stageNames[st.Name]; ok {
			return true
		}
		stageNames[st.Name] = true
	}
	return false
}

func (e *DefaultExecutor) genOpFromSchema(file, stage string, config schema.YipConfig, fs vfs.FS, console plugins.Console) []*op {
	results := []*op{}
	currentStages := config.Stages[stage]

	duplicatedNames := checkDuplicates(currentStages)

	prev := ""
	for i, st := range currentStages {
		name := st.Name
		if duplicatedNames {
			name = fmt.Sprintf("%s.%d", st.Name, i)
		}

		if name == "" {
			name = fmt.Sprint(i)
		}

		rootname := file
		if config.Name != "" {
			rootname = config.Name
		}

		// Copy here so it doesn't get overwritten and points to the same state
		stageLocal := st
		opName := fmt.Sprintf("%s.%s", rootname, name)

		e.logger.Debugf("Generating op for stage '%s'", opName)
		o := &op{
			fn: func(ctx context.Context) error {
				e.logger.Debugf("Reading '%s'", file)
				e.logger.Debugf("Executing stage '%s'", opName)
				return e.applyStage(config, opName, stageLocal, fs, console)
			},
			name:    opName,
			options: []herd.OpOption{herd.WeakDeps},
		}

		for _, d := range st.After {
			o.after = append(o.after, d.Name)
		}

		if i != 0 && len(st.After) == 0 {
			o.deps = append(o.deps, prev)
		}

		results = append(results, o)

		prev = opName
	}

	return results
}

func (e *DefaultExecutor) dirOps(stage, dir string, fs vfs.FS, console plugins.Console) ([]*op, error) {
	results := []*op{}
	prev := []*op{}
	var loadErrors error
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

			// Walk hands us the Lstat, so resolve the entry before deciding:
			// a symlink to a real config is a layout yip supports. Anything
			// that is not a regular file is skipped rather than returned as
			// an error, so a single planted path cannot cost the directory
			// every config next to it (kairos-io/kairos#4865).
			target, err := fs.Stat(path)
			if err != nil {
				e.logger.Warnf("skipping %s: %s", path, err.Error())
				return nil
			}
			if !target.Mode().IsRegular() {
				e.logger.Warnf("skipping %s: it is %s, not a regular file", path, schema.FileTypeName(target.Mode()))
				return nil
			}

			config, err := schema.Load(path, fs, schema.FromFile, e.modifier)
			if err != nil {
				// Same rule as above, for a file yip can open but cannot
				// parse. Returning the error ends the walk and the caller
				// then throws away every op collected so far, so one typo
				// in one config costs the directory all the others, in
				// lexicographic order or not (kairos-io/kairos#5382). The
				// error still travels back, so a strict caller keeps
				// failing on it.
				e.logger.Errorf("skipping %s: %s", path, err.Error())
				loadErrors = multierror.Append(loadErrors, fmt.Errorf("%s: %w", path, err))
				return nil
			}
			ops := e.genOpFromSchema(path, stage, *config, fs, console)
			// mark lexicographic order dependency from previous blocks
			if len(prev) > 0 && len(ops) > 0 {
				for _, p := range prev {
					if len(p.after) == 0 {
						for _, o := range ops {
							o.deps = append(o.deps, p.name)
						}
					}
				}
			}
			prev = ops

			// append results
			results = append(results, ops...)
			return nil
		})
	if err != nil {
		loadErrors = multierror.Append(loadErrors, err)
	}
	return results, loadErrors
}

func writeDAG(dag [][]herd.GraphEntry) {
	for i, layer := range dag {
		fmt.Printf("%d.\n", (i + 1))
		for _, op := range layer {
			if op.Error != nil {
				fmt.Printf(" <%s> (error: %s) (background: %t) (weak: %t)\n", op.Name, op.Error.Error(), op.Background, op.WeakDeps)
			} else {
				fmt.Printf(" <%s> (background: %t) (weak: %t)\n", op.Name, op.Background, op.WeakDeps)
			}
		}
	}
	return
}

func (e *DefaultExecutor) Graph(stage string, fs vfs.FS, console plugins.Console, source string) ([][]herd.GraphEntry, error) {
	g, err := e.prepareDAG(stage, source, fs, console)
	if g == nil {
		return nil, err
	}
	return g.Analyze(), err
}

func (e *DefaultExecutor) Analyze(stage string, fs vfs.FS, console plugins.Console, args ...string) {
	var errs error
	for _, source := range args {
		g, err := e.prepareDAG(stage, source, fs, console)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
		if g == nil {
			continue
		}
		for i, layer := range g.Analyze() {
			e.logger.Infof("%d.", (i + 1))
			for _, op := range layer {
				if op.Error != nil {
					e.logger.Infof(" <%s> (error: %s) (background: %t) (weak: %t)", op.Name, op.Error.Error(), op.Background, op.WeakDeps)
				} else {
					e.logger.Infof(" <%s> (background: %t) (weak: %t)", op.Name, op.Background, op.WeakDeps)
				}
			}
		}
	}
}

func (e *DefaultExecutor) prepareDAG(stage, uri string, fs vfs.FS, console plugins.Console) (*herd.Graph, error) {
	f, err := fs.Stat(uri)

	g := herd.DAG(herd.EnableInit)
	var ops opList
	// A directory is the only source that can fail on part of itself and
	// still carry usable ops. Keep that failure next to the graph instead of
	// in place of it, so the configs that did load still run.
	var partial error
	switch {
	case err == nil && f.IsDir():
		ops, partial = e.dirOps(stage, uri, fs, console)
	case err == nil:
		config, err := schema.Load(uri, fs, schema.FromFile, e.modifier)
		if err != nil {
			return nil, err
		}

		ops = e.genOpFromSchema(uri, stage, *config, fs, console)
	case utils.IsUrl(uri):
		config, err := schema.Load(uri, fs, schema.FromUrl, e.modifier)
		if err != nil {
			return nil, err
		}

		ops = e.genOpFromSchema(uri, stage, *config, fs, console)
	default:
		config, err := schema.Load(uri, fs, nil, e.modifier)
		if err != nil {
			return nil, err
		}

		ops = e.genOpFromSchema("<STDIN>", stage, *config, fs, console)
	}

	// Ensure all names are unique
	ops.uniqueNames()
	for _, o := range ops {
		g.Add(o.name, append(o.options, herd.WithCallback(o.fn), herd.WithDeps(append(o.after, o.deps...)...))...)
	}

	return g, partial
}

func (e *DefaultExecutor) runStage(stage, uri string, fs vfs.FS, console plugins.Console) (err error) {
	g, prepErr := e.prepareDAG(stage, uri, fs, console)
	if g == nil {
		if prepErr != nil {
			return prepErr
		}
		return fmt.Errorf("no dag could be created")
	}

	if prepErr != nil {
		err = multierror.Append(err, prepErr)
	}

	if rerr := g.Run(context.Background()); rerr != nil {
		return multierror.Append(err, rerr)
	}

	for _, g := range g.Analyze() {
		for _, gg := range g {
			if gg.Error != nil {
				err = multierror.Append(err, gg.Error)
			}
		}
	}

	return err
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
			stageName,
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

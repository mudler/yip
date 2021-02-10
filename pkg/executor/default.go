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
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"

	resolvconf "github.com/moby/libnetwork/resolvconf"
	entities "github.com/mudler/entities/pkg/entities"
	log "github.com/sirupsen/logrus"
	"github.com/zcalusic/sysinfo"

	"github.com/hashicorp/go-multierror"
	"github.com/ionrock/procs"
	"github.com/mudler/yip/pkg/schema"
	"github.com/pkg/errors"
	"github.com/twpayne/go-vfs"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
)

var system sysinfo.SysInfo

// renderHelm renders the template string with helm
func renderHelm(template string, values, d map[string]interface{}) (string, error) {
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    "",
			Version: "",
		},
		Templates: []*chart.File{
			{Name: "templates", Data: []byte(template)},
		},
		Values: map[string]interface{}{"Values": values},
	}

	v, err := chartutil.CoalesceValues(c, map[string]interface{}{"Values": d})
	if err != nil {
		return "", errors.Wrap(err, "while rendering template")
	}
	out, err := engine.Render(c, v)
	if err != nil {
		return "", errors.Wrap(err, "while rendering template")
	}

	return out["templates"], nil
}

func templateSysData(s string) string {
	interpolateOpts := map[string]interface{}{}

	data, err := json.Marshal(&system)
	if err != nil {
		log.Warning(fmt.Sprintf("Failed marshalling '%s': %s", s, err.Error()))
		return s
	}
	log.Debug(string(data))

	err = json.Unmarshal(data, &interpolateOpts)
	if err != nil {
		log.Warning(fmt.Sprintf("Failed marshalling '%s': %s", s, err.Error()))
		return s
	}
	rendered, err := renderHelm(s, map[string]interface{}{}, interpolateOpts)
	if err != nil {
		log.Warning(fmt.Sprintf("Failed rendering '%s': %s", s, err.Error()))
		return s
	}
	return rendered
}

// DefaultExecutor is the default yip Executor.
// It simply creates file and executes command for a linux executor
type DefaultExecutor struct{}

func (e *DefaultExecutor) applyDNS(s schema.Stage) error {
	path := s.Dns.Path
	if path == "" {
		path = "/etc/resolv.conf"
	}
	log.Debug("Setting DNS ", path, s.Dns.Nameservers, s.Dns.DnsSearch, s.Dns.DnsOptions)
	_, err := resolvconf.Build(path, s.Dns.Nameservers, s.Dns.DnsSearch, s.Dns.DnsOptions)
	return err
}

func (e *DefaultExecutor) ensureEntities(s schema.Stage) error {
	var errs error
	entityParser := entities.Parser{}
	for _, e := range s.EnsureEntities {
		decodedE, err := entityParser.ReadEntityFromBytes([]byte(templateSysData(e.Entity)))
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

func (e *DefaultExecutor) deleteEntities(s schema.Stage) error {
	var errs error
	entityParser := entities.Parser{}
	for _, e := range s.DeleteEntities {
		decodedE, err := entityParser.ReadEntityFromBytes([]byte(templateSysData(e.Entity)))
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

func (e *DefaultExecutor) writeDirectory(dir schema.Directory, fs vfs.FS) error {
	log.Debug("Creating directory ", dir.Path)
	err := fs.Mkdir(dir.Path, os.FileMode(dir.Permissions))
	if err != nil {
		return err
	}

	return fs.Chown(dir.Path, dir.Owner, dir.Group)
}

func (e *DefaultExecutor) writeFile(file schema.File, fs vfs.FS) error {
	log.Debug("Creating file ", file.Path)
	fsfile, err := fs.Create(file.Path)
	if err != nil {
		return err
	}

	_, err = fsfile.WriteString(templateSysData(file.Content))
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
	log.Info(fmt.Sprintf("Running command: '%s'", cmd))
	p := procs.NewProcess(templateSysData(cmd))
	err := p.Run()
	if err != nil {
		return "", err
	}
	out, err := p.Output()
	return string(out), err
}

func (e *DefaultExecutor) Walk(stage string, args []string, fs vfs.FS) error {
	var errs error
	var config *schema.YipConfig

	for _, source := range args {
		// Load yamls in a directory
		if f, err := fs.Stat(source); err == nil && f.IsDir() {
			err := vfs.Walk(fs, source,
				func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if path == source {
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
					config, err = schema.LoadFromFile(path, fs)
					if err != nil {
						errs = multierror.Append(errs, err)
						return nil
					}
					log.Infof("Executing %s", path)
					if err = e.Apply(stage, *config, vfs.OSFS); err != nil {
						errs = multierror.Append(errs, err)
						return nil
					}

					return nil
				})
			if err != nil {
				errs = multierror.Append(errs, err)
			}

			continue
		}

		// Parse urls/file
		_, err := url.ParseRequestURI(source)
		if err != nil {
			config, err = schema.LoadFromFile(source, fs)
		} else {
			config, err = schema.LoadFromUrl(source)
		}

		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}

		if err = e.Apply(stage, *config, fs); err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
	}
	return errs
}

// Apply applies a yip Config file by creating files and running commands defined.
func (e *DefaultExecutor) Apply(stageName string, s schema.YipConfig, fs vfs.FS) error {
	system.GetSysInfo()

	currentStages, _ := s.Stages[stageName]
	var errs error

	log.WithFields(log.Fields{
		"name":   s.Name,
		"stages": len(currentStages),
		"stage":  stageName,
	}).Info("Executing yip file")
	for _, stage := range currentStages {

		if len(stage.Node) > 0 {
			matched, err := regexp.MatchString(stage.Node, system.Node.Hostname)
			if !matched {
				log.WithFields(log.Fields{
					"name":  s.Name,
					"stage": stageName,
				}).Info(fmt.Sprintf("Skipping stage (node hostname '%s' doesn't match '%s')", system.Node.Hostname, stage.Node))
				continue
			}
			if err != nil {
				log.WithFields(log.Fields{
					"name":  s.Name,
					"stage": stageName,
				}).Warning(fmt.Sprintf("Skipping invalid regex for node hostname '%s', error: %s", stage.Node, err.Error()))
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
		if len(stage.Dns.Nameservers) != 0 {
			if err := e.applyDNS(stage); err != nil {
				log.Error(err.Error())
				errs = multierror.Append(errs, err)
			}
		}

		if len(stage.EnsureEntities) > 0 {
			if err := e.ensureEntities(stage); err != nil {
				log.Error(err.Error())
				errs = multierror.Append(errs, err)
			}
		}

		for _, dir := range stage.Directories {
			if err := e.writeDirectory(dir, fs); err != nil {
				log.Error(err.Error())
				errs = multierror.Append(errs, err)
				continue
			}
		}

		for _, file := range stage.Files {
			if err := e.writeFile(file, fs); err != nil {
				log.Error(err.Error())
				errs = multierror.Append(errs, err)
				continue
			}
		}

		for _, cmd := range stage.Commands {
			out, err := e.runProc(cmd)
			if err != nil {
				log.Error(err.Error())
				errs = multierror.Append(errs, err)
				continue
			}
			log.Info(fmt.Sprintf("Command output: %s", string(out)))
		}
		if len(stage.DeleteEntities) > 0 {
			if err := e.deleteEntities(stage); err != nil {
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

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

package schema

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/itchyny/gojq"
	"github.com/twpayne/go-vfs"
	"gopkg.in/yaml.v2"
)

type YipEntity struct {
	Path   string `yaml:"path"`
	Entity string `yaml:"entity"`
}

type File struct {
	Path         string
	Permissions  uint32
	Owner, Group int
	Content      string
}

type Directory struct {
	Path         string
	Permissions  uint32
	Owner, Group int
}

type Stage struct {
	Commands    []string    `yaml:"commands"`
	Files       []File      `yaml:"files"`
	Directories []Directory `yaml:"directories"`

	EnsureEntities  []YipEntity         `yaml:"ensure_entities"`
	DeleteEntities  []YipEntity         `yaml:"delete_entities"`
	Dns             DNS                 `yaml:"dns"`
	Hostname        string              `yaml:"hostname"`
	Name            string              `yaml:"name"`
	Sysctl          map[string]string   `yaml:"sysctl"`
	SSHKeys         map[string][]string `yaml:"authorized_keys"`
	Node            string              `yaml:"node"`
	Users           map[string]string   `yaml:"users"`
	Modules         []string            `yaml:"modules"`
	Systemctl       Systemctl           `yaml:"systemctl"`
	Environment     map[string]string   `yaml:"environment"`
	EnvironmentFile string              `yaml:"environment_file"`

	TimeSyncd map[string]string `yaml:"timesyncd"`
}

type Systemctl struct {
	Enable  []string `yaml:"enable"`
	Disable []string `yaml:"disable"`
	Start   []string `yaml:"start"`
	Mask    []string `yaml:"mask"`
}

type DNS struct {
	Nameservers []string `yaml:"nameservers"`
	DnsSearch   []string `yaml:"search"`
	DnsOptions  []string `yaml:"options"`
	Path        string   `yaml:"path"`
}

type YipConfig struct {
	Name   string             `yaml:"name"`
	Stages map[string][]Stage `yaml:"stages"`
}

// LoadFromFile loads a yip config from a YAML file
func LoadFromFile(s string, fs vfs.FS) (*YipConfig, error) {
	yamlFile, err := fs.ReadFile(s)
	if err != nil {
		return nil, err
	}

	return LoadFromYaml(yamlFile)
}

// LoadFromYaml loads a yip config from bytes
func LoadFromYaml(b []byte) (*YipConfig, error) {

	var yamlConfig YipConfig
	err := yaml.Unmarshal(b, &yamlConfig)
	if err != nil {
		return nil, err
	}

	return &yamlConfig, nil
}

// LoadFromUrl loads a yip config from a url
func LoadFromUrl(s string) (*YipConfig, error) {
	resp, err := http.Get(s)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	buf := bytes.NewBuffer([]byte{})
	_, err = io.Copy(buf, resp.Body)

	return LoadFromYaml(buf.Bytes())
}

func jq(command string, data map[string]interface{}) (map[string]interface{}, error) {
	query, err := gojq.Parse(command)
	if err != nil {
		return nil, err
	}
	code, err := gojq.Compile(query)
	if err != nil {
		return nil, err
	}
	iter := code.Run(data)

	v, ok := iter.Next()
	if !ok {
		return nil, errors.New("failed getting rsult from gojq")
	}
	if err, ok := v.(error); ok {
		return nil, err
	}
	return v.(map[string]interface{}), nil
}

func dotToYAML(v map[string]interface{}) ([]byte, error) {
	data := map[string]interface{}{}
	var errs error

	for k, value := range v {
		newData, err := jq(fmt.Sprintf(".%s=\"%s\"", k, value), data)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
		data = newData
	}

	out, err := yaml.Marshal(&data)
	if err != nil {
		errs = multierror.Append(errs, err)
	}
	return out, err
}

func LoadFromDotNotation(v map[string]interface{}) (*YipConfig, error) {
	data, err := dotToYAML(v)
	if err != nil {
		return nil, err
	}
	return LoadFromYaml(data)
}

// LoadFromDotNotationS read a string in dot notation
// e.g. foo.bar=boo
func LoadFromDotNotationS(s string) (*YipConfig, error) {
	v := map[string]interface{}{}

	for _, item := range strings.Fields(s) {
		parts := strings.SplitN(item, "=", 2)
		value := "true"
		if len(parts) > 1 {
			value = strings.Trim(parts[1], `"`)
		}
		key := strings.Trim(parts[0], `"`)
		v[key] = value
	}

	data, err := dotToYAML(v)
	if err != nil {
		return nil, err
	}
	return LoadFromYaml(data)
}

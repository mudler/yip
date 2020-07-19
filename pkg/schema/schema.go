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
	"io"
	"io/ioutil"
	"net/http"

	"gopkg.in/yaml.v2"
)

type File struct {
	Path         string
	Permissions  uint32
	Owner, Group int
	Content      string
}

type Stage struct {
	Commands []string `yaml:"commands"`
	Files    []File   `yaml:"files"`
}

type DNS struct {
	Nameservers []string `yaml:"nameservers"`
	DnsSearch   []string `yaml:"search"`
	DnsOptions  []string `yaml:"options"`
	Path        string   `yaml:"path"`
}

type YipConfig struct {
	Stages map[string][]Stage `yaml:"stages"`
	Dns    DNS                `yaml:"dns"`
}

// LoadFromFile loads a yip config from a YAML file
func LoadFromFile(s string) (*YipConfig, error) {
	yamlFile, err := ioutil.ReadFile(s)
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

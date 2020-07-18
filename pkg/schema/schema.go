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

type EApplyConfig struct {
	Stages map[string][]Stage `yaml:"stages"`
}

func LoadFromFile(s string) (*EApplyConfig, error) {
	yamlFile, err := ioutil.ReadFile(s)
	if err != nil {
		return nil, err
	}

	var yamlConfig EApplyConfig
	err = yaml.Unmarshal(yamlFile, &yamlConfig)
	if err != nil {
		return nil, err
	}

	return &yamlConfig, nil
}

func LoadFromUrl(s string) (*EApplyConfig, error) {
	resp, err := http.Get(s)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	buf := bytes.NewBuffer([]byte{})
	_, err = io.Copy(buf, resp.Body)

	var yamlConfig EApplyConfig
	err = yaml.Unmarshal(buf.Bytes(), &yamlConfig)
	if err != nil {
		return nil, err
	}

	return &yamlConfig, nil
}

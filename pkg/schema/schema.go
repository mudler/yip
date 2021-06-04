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
	"fmt"
	"io"
	"net/http"
	"os/user"
	"strings"

	"github.com/elotl/cloud-init/config"
	"github.com/pkg/errors"

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
	Encoding     string
	OwnerString  string
}

type Directory struct {
	Path         string
	Permissions  uint32
	Owner, Group int
}

type DataSource struct {
	Providers []string `yaml:"providers"`
	Path      string   `yaml:"path"`
}

type User struct {
	Name              string   `yaml:"name,omitempty"`
	PasswordHash      string   `yaml:"passwd,omitempty"`
	SSHAuthorizedKeys []string `yaml:"ssh_authorized_keys,omitempty"`
	GECOS             string   `yaml:"gecos,omitempty"`
	Homedir           string   `yaml:"homedir,omitempty"`
	NoCreateHome      bool     `yaml:"no_create_home,omitempty"`
	PrimaryGroup      string   `yaml:"primary_group,omitempty"`
	Groups            []string `yaml:"groups,omitempty"`
	NoUserGroup       bool     `yaml:"no_user_group,omitempty"`
	System            bool     `yaml:"system,omitempty"`
	NoLogInit         bool     `yaml:"no_log_init,omitempty"`
	Shell             string   `yaml:"shell,omitempty"`
}

func (u User) Exists() bool {
	_, err := user.Lookup(u.Name)
	return err == nil
}

type Layout struct {
	Device *Device     `yaml:"device"`
	Expand *Expand     `yaml:"expand_partition,omitempty"`
	Parts  []Partition `yaml:"add_partitions,omitempty"`
}

type Device struct {
	Label string `"yaml:label"`
}

type Expand struct {
	Size uint `"yaml:size"`
}

type Partition struct {
	FSLabel    string `yaml:"fsLabel"`
	Size       uint   `yaml:"size,omitempty"`
	PLabel     string `yaml:"pLabel,omitempty"`
	FileSystem string `yaml:"filesystem,omitempty"`
}

type Stage struct {
	Commands    []string    `yaml:"commands"`
	Files       []File      `yaml:"files"`
	Directories []Directory `yaml:"directories"`
	If          string      `yaml:"if"`

	EnsureEntities  []YipEntity         `yaml:"ensure_entities"`
	DeleteEntities  []YipEntity         `yaml:"delete_entities"`
	Dns             DNS                 `yaml:"dns"`
	Hostname        string              `yaml:"hostname"`
	Name            string              `yaml:"name"`
	Sysctl          map[string]string   `yaml:"sysctl"`
	SSHKeys         map[string][]string `yaml:"authorized_keys"`
	Node            string              `yaml:"node"`
	Users           map[string]User     `yaml:"users"`
	Modules         []string            `yaml:"modules"`
	Systemctl       Systemctl           `yaml:"systemctl"`
	Environment     map[string]string   `yaml:"environment"`
	EnvironmentFile string              `yaml:"environment_file"`

	DataSources DataSource `yaml:"datasource"`
	Layout      Layout     `yaml:"layout"`

	SystemdFirstBoot map[string]string `yaml:"systemd_firstboot"`

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

type Loader func(s string, fs vfs.FS, m Modifier) ([]byte, error)
type Modifier func(s []byte) ([]byte, error)

type yipLoader interface {
	Load([]byte, vfs.FS) (*YipConfig, error)
}

func Load(s string, fs vfs.FS, l Loader, m Modifier) (*YipConfig, error) {
	if m == nil {
		m = func(b []byte) ([]byte, error) { return b, nil }
	}
	if l == nil {
		l = func(c string, fs vfs.FS, m Modifier) ([]byte, error) { return m([]byte(c)) }
	}
	data, err := l(s, fs, m)
	if err != nil {
		return nil, errors.Wrap(err, "while loading yipconfig")
	}

	loader, err := detect(data)
	if err != nil {
		return nil, errors.Wrap(err, "invalid file type")
	}
	return loader.Load(data, fs)
}

func detect(b []byte) (yipLoader, error) {
	switch {
	case config.IsCloudConfig(string(b)):
		return cloudInit{}, nil

	default:
		return yipYAML{}, nil
	}
}

// FromFile loads a yip config from a YAML file
func FromFile(s string, fs vfs.FS, m Modifier) ([]byte, error) {
	yamlFile, err := fs.ReadFile(s)
	if err != nil {
		return nil, err
	}
	return m(yamlFile)
}

// FromUrl loads a yip config from a url
func FromUrl(s string, fs vfs.FS, m Modifier) ([]byte, error) {
	resp, err := http.Get(s)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	buf := bytes.NewBuffer([]byte{})
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return nil, err
	}
	return m(buf.Bytes())
}

// DotNotationModifier read a byte sequence in dot notation and returns a byte sequence in yaml
// e.g. foo.bar=boo
func DotNotationModifier(s []byte) ([]byte, error) {
	v := stringToMap(string(s))

	data, err := dotToYAML(v)
	if err != nil {
		return nil, err
	}
	return data, nil
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

func stringToMap(s string) map[string]interface{} {
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

	return v
}

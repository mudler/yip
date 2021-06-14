// Copyright © 2021 Ettore Di Giacinto <mudler@sabayon.org>
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
	cloudconfig "github.com/elotl/cloud-init/config"
	"github.com/twpayne/go-vfs"
)

type cloudInit struct{}

// Load transpiles a cloud-init style
// file ( https://cloudinit.readthedocs.io/en/latest/topics/examples.html)
// to a yip schema.
// As Yip supports multi-stages, it is encoded in the supplied one.
// fs is used to parse the user data required from /etc/passwd.
func (cloudInit) Load(s []byte, fs vfs.FS) (*YipConfig, error) {
	stage := "boot"
	cc, err := cloudconfig.NewCloudConfig(string(s))
	if err != nil {
		return nil, err
	}

	// Decode users and SSH Keys
	sshKeys := make(map[string][]string)
	users := make(map[string]User)
	userstoKey := []string{}

	for _, u := range cc.Users {
		userstoKey = append(userstoKey, u.Name)
		users[u.Name] = User{
			Name:         u.Name,
			PasswordHash: u.PasswordHash,
			GECOS:        u.GECOS,
			Homedir:      u.Homedir,
			NoCreateHome: u.NoCreateHome,
			PrimaryGroup: u.PrimaryGroup,
			Groups:       u.Groups,
			NoUserGroup:  u.NoUserGroup,
			System:       u.System,
			NoLogInit:    u.NoLogInit,
			Shell:        u.Shell,
		}
		sshKeys[u.Name] = u.SSHAuthorizedKeys
	}

	for _, uu := range userstoKey {
		_, exists := sshKeys[uu]
		if !exists {
			sshKeys[uu] = cc.SSHAuthorizedKeys
		} else {
			sshKeys[uu] = append(sshKeys[uu], cc.SSHAuthorizedKeys...)
		}
	}

	// Decode writeFiles
	var f []File
	for _, ff := range cc.WriteFiles {
		f = append(f,
			File{
				Path:        ff.Path,
				OwnerString: ff.Owner,
				Content:     ff.Content,
				Encoding:    ff.Encoding,
			},
		)
	}

	for _, ff := range cc.MilpaFiles {
		f = append(f,
			File{
				Path:        ff.Path,
				OwnerString: ff.Owner,
				Content:     ff.Content,
				Encoding:    ff.Encoding,
			},
		)
	}

	result := &YipConfig{
		Name: "Cloud init",
		Stages: map[string][]Stage{stage: {{
			Commands: cc.RunCmd,
			Files:    f,
			Hostname: cc.Hostname,
			Users:    users,
			SSHKeys:  sshKeys,
		}}},
	}

	// optimistically load data as yip yaml
	yipConfig, err := yipYAML{}.Load(s, fs)
	if err == nil {
		for k, v := range yipConfig.Stages {
			result.Stages[k] = append(result.Stages[k], v...)
		}
	}

	return result, nil
}

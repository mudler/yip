//   Copyright 2022 Itxaka Serrano <itxakaserrano@gmail.com>
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

package plugins

import (
	"fmt"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs"
	"gopkg.in/ini.v1"
	"math/rand"
	"path/filepath"
)

const ConnectionsDir = "/etc/NetworkManager/system-connections/"

func NetworkManager(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	for _, conn := range s.NetworkManager {
		cfg, err := conn.ToInifile()
		if err != nil {
			return err
		}
		if conn.Name == "" {
			conn.Name = name(cfg)
		}
		finalFilePath := filepath.Join(ConnectionsDir, fmt.Sprintf("%s.connection", conn.Name))
		l.Infof("Saving connection %s to file %s", conn.Name, finalFilePath)
		f, err := fs.Create(finalFilePath)
		if err != nil {
			return err
		}
		_, err = cfg.WriteTo(f)
		if err != nil {
			return err
		}
	}

	return nil
}

func name(cfg *ini.File) string {
	if cfg.Section("connection").HasKey("id") {
		// Connection has id, use that as name
		return cfg.Section("connection").Key("id").String()
	} else {
		// No id, get the interface
		iface := cfg.Section("connection").Key("interface-name")
		// Get a random number of 4 digits. How absurd is this? Why isn't there a better interface to a random number in a range?
		min := 1000
		max := 9999
		randomInt := rand.Intn(max-min+1) + min
		return fmt.Sprintf("%s-%d", iface, randomInt)
	}
}

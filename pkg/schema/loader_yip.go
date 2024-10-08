package schema

import (
	"github.com/twpayne/go-vfs/v4"
	"gopkg.in/yaml.v2"
)

type yipYAML struct{}

// LoadFromYaml loads a yip config from bytes
func (yipYAML) Load(source string, b []byte, fs vfs.FS) (*YipConfig, error) {
	var yamlConfig YipConfig
	err := yaml.Unmarshal(b, &yamlConfig)
	if err != nil {
		return nil, err
	}
	yamlConfig.Source = source
	return &yamlConfig, nil
}

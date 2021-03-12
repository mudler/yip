package plugins

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/mudler/yip/pkg/schema"
	"github.com/mudler/yip/pkg/utils"
	"github.com/pkg/errors"
	"github.com/twpayne/go-vfs"
)

const environmentFile = "/etc/environment"

func Environment(s schema.Stage, fs vfs.FS, console Console) error {
	if len(s.Environment) == 0 {
		return nil
	}
	environment := s.EnvironmentFile
	if environment == "" {
		environment = environmentFile
	}

	if err := utils.Touch(environment, os.ModePerm, fs); err != nil {
		return errors.Wrap(err, "failed touching environment file")
	}

	content, err := fs.ReadFile(environment)
	if err != nil {
		return err
	}
	env, err := godotenv.Unmarshal(string(content))

	for key, val := range s.Environment {
		env[key] = val
	}

	p, err := fs.RawPath(environment)
	if err != nil {
		return err
	}
	err = godotenv.Write(env, p)
	if err != nil {
		return err
	}

	return fs.Chmod(environment, 0644)
}

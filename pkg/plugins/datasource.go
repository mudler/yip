package plugins

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"

	"github.com/pkg/errors"

	prv "github.com/davidcassany/linuxkit/pkg/metadata/providers"
	"github.com/mudler/yip/pkg/schema"
	log "github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs"
)

func DataSources(s schema.Stage, fs vfs.FS, console Console) error {
	var AvailableProviders = []prv.Provider{}

	if len(s.DataSources) == 0 {
		return nil
	}

	for _, ds := range s.DataSources {
		switch {
		case ds.Type == "aws":
			AvailableProviders = append(AvailableProviders, prv.NewAWS())
		case ds.Type == "gcp":
			AvailableProviders = append(AvailableProviders, prv.NewGCP())
		case ds.Type == "hetzner":
			AvailableProviders = append(AvailableProviders, prv.NewHetzner())
		case ds.Type == "openstack":
			AvailableProviders = append(AvailableProviders, prv.NewOpenstack())
		case ds.Type == "packet":
			AvailableProviders = append(AvailableProviders, prv.NewPacket())
		case ds.Type == "scaleway":
			AvailableProviders = append(AvailableProviders, prv.NewScaleway())
		case ds.Type == "vultr":
			AvailableProviders = append(AvailableProviders, prv.NewVultr())
		case ds.Type == "digitalocean":
			AvailableProviders = append(AvailableProviders, prv.NewDigitalOcean())
		case ds.Type == "metaldata":
			AvailableProviders = append(AvailableProviders, prv.NewMetalData())
		case ds.Type == "cdrom":
			AvailableProviders = append(AvailableProviders, prv.ListCDROMs()...)
		case ds.Type == "file":
			AvailableProviders = append(AvailableProviders, prv.FileProvider(ds.Path))
		}
	}

	var p prv.Provider
	var userdata []byte
	var err error
	found := false
	for _, p = range AvailableProviders {
		if p.Probe() {
			userdata, err = p.Extract()
			if err != nil {
				log.Warningf("Failed extracting data from %s provider: %s", p.String(), err.Error())
			}
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("No metadata/userdata found. Bye")
	}

	err = EnsureFiles(schema.Stage{
		Files: []schema.File{
			{
				Path:        path.Join(prv.ConfigPath, "provider"),
				Content:     p.String(),
				Permissions: 0644,
				Owner:       os.Getuid(),
				Group:       os.Getgid(),
			},
		},
	}, fs, console)
	if err != nil {
		return err
	}

	if userdata != nil {
		if err := processUserData(prv.ConfigPath, userdata, fs, console); err != nil {
			return err
		}
	}
	if _, err := os.Stat(path.Join(prv.ConfigPath, prv.Hostname)); err == nil {
		hostname, err := ioutil.ReadFile(path.Join(prv.ConfigPath, prv.Hostname))
		if err != nil {
			return err
		}

		return Hostname(schema.Stage{Hostname: string(hostname)}, fs, console)
	}
	return nil
}

// If the userdata is a json file, create a directory/file hierarchy.
// Example:
// {
//    "foobar" : {
//        "foo" : {
//            "perm": "0644",
//            "content": "hello"
//        }
// }
// Will create foobar/foo with mode 0644 and content "hello"
func processUserData(basePath string, data []byte, fs vfs.FS, console Console) error {

	// Always write the raw data to a file
	err := EnsureFiles(schema.Stage{
		Files: []schema.File{
			{
				Path:        path.Join(basePath, "userdata"),
				Content:     string(data),
				Permissions: 0644,
				Owner:       os.Getuid(),
				Group:       os.Getgid(),
			},
		},
	}, fs, console)
	if err != nil {
		return errors.Wrap(err, "could not write userdata")
	}

	var root ConfigFile
	if err := json.Unmarshal(data, &root); err != nil {
		// Userdata is no JSON, presumably...
		log.Printf("Could not unmarshall userdata: %s", err)
		// This is not an error
		return nil
	}

	for dir, entry := range root {
		writeConfigFiles(path.Join(basePath, dir), entry, fs, console)
	}
	return nil
}

func writeConfigFiles(target string, current Entry, fs vfs.FS, console Console) {
	if isFile(current) {
		filemode, err := parseFileMode(current.Perm, 0644)
		if err != nil {
			log.Printf("Failed to parse permission %+v: %s", current, err)
			return
		}

		if err := EnsureFiles(schema.Stage{
			Files: []schema.File{
				{
					Path:        target,
					Content:     *current.Content,
					Permissions: uint32(filemode.Perm()),
					Owner:       os.Getuid(),
					Group:       os.Getgid(),
				},
			},
		}, fs, console); err != nil {
			log.Printf("Failed to write %s: %s", target, err)
			return
		}
	} else if isDirectory(current) {
		filemode, err := parseFileMode(current.Perm, 0755)
		if err != nil {
			log.Printf("Failed to parse permission %+v: %s", current, err)
			return
		}
		if err := EnsureDirectories(schema.Stage{
			Directories: []schema.Directory{
				{
					Path:        target,
					Permissions: uint32(filemode.Perm()),
					Owner:       os.Getuid(),
					Group:       os.Getgid(),
				},
			},
		}, fs, console); err != nil {
			log.Printf("Failed to write %s: %s", target, err)
			return
		}

		for dir, entry := range current.Entries {
			writeConfigFiles(path.Join(target, dir), entry, fs, console)
		}
	} else {
		log.Printf("%s is invalid", target)
	}
}

func isFile(json Entry) bool {
	return json.Content != nil && json.Entries == nil
}

func isDirectory(json Entry) bool {
	return json.Content == nil && json.Entries != nil
}

func parseFileMode(input string, defaultMode os.FileMode) (os.FileMode, error) {
	if input != "" {
		perm, err := strconv.ParseUint(input, 8, 32)
		if err != nil {
			return 0, err
		}
		return os.FileMode(perm), nil
	}
	return defaultMode, nil
}

// ConfigFile represents the configuration file
type ConfigFile map[string]Entry

// Entry represents either a directory or a file
type Entry struct {
	Perm    string           `json:"perm,omitempty"`
	Content *string          `json:"content,omitempty"`
	Entries map[string]Entry `json:"entries,omitempty"`
}

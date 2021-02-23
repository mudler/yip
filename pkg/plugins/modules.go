package plugins

import (
	"bufio"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/schema"
	"github.com/paultag/go-modprobe"
	log "github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs"
)

const (
	modules = "/proc/modules"
)

func loadedModules(fs vfs.FS) map[string]interface{} {
	loaded := map[string]interface{}{}
	f, err := fs.Open(modules)
	if err != nil {
		log.Warningf("Cannot open %s: %s", modules, err.Error())
		return loaded
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		mod := strings.SplitN(sc.Text(), " ", 2)
		if len(mod) == 0 {
			continue
		}
		loaded[mod[0]] = nil
	}
	return loaded
}

func LoadModules(s schema.Stage, fs vfs.FS, console Console) error {
	var errs error

	if len(s.Modules) == 0 {
		return nil
	}

	loaded := loadedModules(fs)

	for _, m := range s.Modules {
		if _, ok := loaded[m]; ok {
			continue
		}
		params := strings.SplitN(m, " ", -1)
		log.Debugf("loading module %s with parameters [%s]", m, params)
		if err := modprobe.Load(params[0], strings.Join(params[1:], " ")); err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
		log.Debugf("module %s loaded", m)
	}
	return errs
}

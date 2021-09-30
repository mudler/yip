package plugins

import (
	"net/http"
	"os"
	"time"

	"github.com/cavaliergopher/grab"
	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/schema"
	"github.com/mudler/yip/pkg/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs"
)

func grabClient(timeout int) *grab.Client {
	return &grab.Client{
		UserAgent: "grab",
		HTTPClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
		},
	}
}

func Download(s schema.Stage, fs vfs.FS, console Console) error {
	var errs error
	for _, dl := range s.Downloads {
		d := &dl
		realPath, err := fs.RawPath(d.Path)
		if err == nil {
			d.Path = realPath
		}
		if err := downloadFile(*d); err != nil {
			log.Error(err.Error())
			errs = multierror.Append(errs, err)
			continue
		}
	}
	return errs
}

func downloadFile(dl schema.Download) error {
	log.Debug("Downloading file ", dl.Path, dl.URL)
	resp, err := grab.Get(dl.Path, dl.URL)
	if err != nil {
		log.Fatal(err)
	}

	file := resp.Filename
	err = os.Chmod(file, os.FileMode(dl.Permissions))
	if err != nil {
		return err

	}

	if dl.OwnerString != "" {
		// FIXUP: Doesn't support fs. It reads real /etc/passwd files
		uid, gid, err := utils.GetUserDataFromString(dl.OwnerString)
		if err != nil {
			return errors.Wrap(err, "Failed getting gid")
		}
		return os.Chown(dl.Path, uid, gid)
	}

	return os.Chown(file, dl.Owner, dl.Group)
}

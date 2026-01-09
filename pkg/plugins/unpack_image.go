//go:build !nounpack

package plugins

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"syscall"

	"github.com/containerd/containerd/archive"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs/v4"
)

func UnpackImage(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	var errs *multierror.Error

	if len(s.UnpackImages) == 0 {
		return nil
	}
	for _, imageConf := range s.UnpackImages {
		if imageConf.Source == "" {
			l.Warn("No source defined for unpack_image")
			continue
		}
		if imageConf.Target == "" {
			l.Warn("No target defined for unpack_image")
			continue
		}
		// create the target directory if it doesnt exist
		if err := mkdirAll(fs, imageConf.Target, 0755); err != nil {
			l.Errorf("Error creating target directory for unpack_image: %w", err)
			errs = multierror.Append(errs, err)
			continue
		}
		// unpack the image
		image, err := getImage(imageConf.Source, imageConf.Platform)
		if err != nil {
			l.Errorf("Error getting image for unpack_image: %w", err)
			errs = multierror.Append(errs, err)
			continue
		}
		if err := extractOCIImage(image, imageConf.Target); err != nil {
			l.Errorf("Error extracting image for unpack_image: %w", err)
			errs = multierror.Append(errs, err)
			continue
		}
	}
	return errs.ErrorOrNil()
}

// ExtractOCIImage will extract a given targetImage into a given targetDestination
func extractOCIImage(img v1.Image, targetDestination string) error {
	reader := mutate.Extract(img)

	_, err := archive.Apply(context.Background(), targetDestination, reader)

	return err
}

func getImage(targetImage, targetPlatform string) (v1.Image, error) {
	var platform *v1.Platform
	var image v1.Image
	var err error

	if targetPlatform != "" {
		platform, err = v1.ParsePlatform(targetPlatform)
		if err != nil {
			return image, err
		}
	} else {
		platform, err = v1.ParsePlatform(fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))
		if err != nil {
			return image, err
		}
	}

	ref, err := name.ParseReference(targetImage)
	if err != nil {
		return image, err
	}

	tr := transport.NewRetry(http.DefaultTransport)

	// Try to get the image from the local Docker daemon
	image, err = daemon.Image(ref)
	if err == nil {
		// Check if the image matches the requested platform
		imgConfig, err := image.ConfigFile()
		if err == nil && imgConfig.Architecture == platform.Architecture && imgConfig.OS == platform.OS {
			return image, nil
		}
	}

	// If the image is not in the local Docker daemon, or does not match the platform try to get it from the registry
	opts := []remote.Option{
		remote.WithTransport(tr),
		remote.WithPlatform(*platform),
	}

	opts = append(opts, remote.WithAuthFromKeychain(authn.DefaultKeychain))

	image, err = remote.Image(ref, opts...)

	return image, err
}

// mkdirAll creates a directory and all necessary parents.
// It uses an vfs.FS as that doesnt have a MkdirAll method
// Same as os.MkdirAll but for vfs.FS
func mkdirAll(fs vfs.FS, path string, perm os.FileMode) error {
	// Fast path: if we can tell whether path is a directory or file, stop with success or error.
	dir, err := fs.Stat(path)
	if err == nil {
		if dir.IsDir() {
			return nil
		}
		return &os.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
	}

	// Slow path: make sure parent exists and then call Mkdir for path.

	// Extract the parent folder from path by first removing any trailing
	// path separator and then scanning backward until finding a path
	// separator or reaching the beginning of the string.
	i := len(path) - 1
	for i >= 0 && os.IsPathSeparator(path[i]) {
		i--
	}
	for i >= 0 && !os.IsPathSeparator(path[i]) {
		i--
	}
	if i < 0 {
		i = 0
	}

	// If there is a parent directory, and it is not the volume name,
	// recurse to ensure parent directory exists.
	if parent := path[:i]; len(parent) > 0 {
		err = mkdirAll(fs, parent, perm)
		if err != nil {
			return err
		}
	}

	// Parent now exists; invoke Mkdir and use its result.
	err = fs.Mkdir(path, perm)
	if err != nil {
		// Handle arguments like "foo/." by
		// double-checking that directory doesn't exist.
		dir, err1 := fs.Lstat(path)
		if err1 == nil && dir.IsDir() {
			return nil
		}
		return err
	}
	return nil
}

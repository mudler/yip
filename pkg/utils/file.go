package utils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"github.com/twpayne/go-vfs/v4"
)

func Touch(s string, perms os.FileMode, fs vfs.FS) error {
	_, err := fs.Stat(s)

	switch {
	case os.IsNotExist(err):
		f, err := fs.Create(s)
		if err != nil {
			return err
		}
		if err = f.Chmod(perms); err != nil {
			return err
		}
		if err = f.Close(); err != nil {
			return err
		}
		_, err = fs.Stat(s)
		return err
	case err == nil:
		return nil
	default:
		return errors.New("could not create file")
	}

}

func Exists(s string) bool {
	if _, err := os.Stat(s); err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}

func WriteTempFile(data []byte, prefix string) (string, error) {
	dir := os.TempDir()
	randBytes := make([]byte, 8)
	_, err := rand.Read(randBytes)
	if err != nil {
		return "", err
	}
	name := prefix + hex.EncodeToString(randBytes)
	path := filepath.Join(dir, name)
	err = os.WriteFile(path, data, 0600)
	if err != nil {
		return "", err
	}
	return path, nil
}

func RemoveFile(path string) error {
	return os.Remove(path)
}

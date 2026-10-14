package plugins

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs/v5"
)

func DNS(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	if len(s.Dns.Nameservers) != 0 {
		return applyDNS(s, fs)
	}
	return nil
}

func applyDNS(s schema.Stage, fs vfs.FS) error {
	path := s.Dns.Path
	if path == "" {
		path = "/etc/resolv.conf"
	}
	return Build(path, s.Dns.Nameservers, s.Dns.DnsSearch, s.Dns.DnsOptions, fs)
}

func Build(path string, nameservers, dnsSearch, dnsOptions []string, fs vfs.FS) error {
	content := bytes.NewBuffer(nil)
	if len(dnsSearch) > 0 {
		if searchString := strings.Join(dnsSearch, " "); strings.Trim(searchString, " ") != "." {
			if _, err := content.WriteString("search " + searchString + "\n"); err != nil {
				return err
			}
		}
	}
	for _, dns := range nameservers {
		if _, err := content.WriteString("nameserver " + dns + "\n"); err != nil {
			return err
		}
	}
	if len(dnsOptions) > 0 {
		if optsString := strings.Join(dnsOptions, " "); strings.Trim(optsString, " ") != "" {
			if _, err := content.WriteString("options " + optsString + "\n"); err != nil {
				return err
			}
		}
	}

	dir := filepath.Dir(path)
	if err := vfs.MkdirAll(fs, dir, 0755); err != nil {
		return err
	}

	// Most systemd distributions ship /etc/resolv.conf as a symlink into
	// systemd-resolved's runtime directory, and Kairos creates that link itself.
	// Writing through it lands the nameservers in a file resolved owns and
	// regenerates, on a tmpfs that does not exist yet in the initramfs. Write a
	// sibling and rename over the path instead: rename acts on the link rather
	// than its target, so the caller gets a regular file where it asked for one
	// and no reader ever sees a half-written resolv.conf.
	tmp := filepath.Join(dir, "."+filepath.Base(path)+".yip")
	if err := fs.WriteFile(tmp, content.Bytes(), 0644); err != nil {
		return err
	}
	if err := fs.Rename(tmp, path); err != nil {
		fs.Remove(tmp)
		return err
	}
	return nil
}

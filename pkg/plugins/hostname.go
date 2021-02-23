package plugins

import (
	"bufio"
	"fmt"
	"strings"
	"syscall"

	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs"
)

const localHost = "127.0.0.1"

func Hostname(s schema.Stage, fs vfs.FS, console Console) error {
	var errs error
	hostname := s.Hostname
	if hostname == "" {
		return nil
	}
	if err := syscall.Sethostname([]byte(hostname)); err != nil {
		errs = multierror.Append(errs, err)
	}
	if err := SystemHostname(hostname, fs); err != nil {
		errs = multierror.Append(errs, err)
	}
	if err := UpdateHostsFile(hostname, fs); err != nil {
		errs = multierror.Append(errs, err)
	}
	return errs
}

func UpdateHostsFile(hostname string, fs vfs.FS) error {
	hosts, err := fs.Open("/etc/hosts")
	defer hosts.Close()
	if err != nil {
		return err
	}
	lines := bufio.NewScanner(hosts)
	content := ""
	for lines.Scan() {
		line := strings.TrimSpace(lines.Text())
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == localHost {
			content += fmt.Sprintf("%s %s\n", localHost, hostname)
			continue
		}
		content += line + "\n"
	}
	return fs.WriteFile("/etc/hosts", []byte(content), 0600)
}

func SystemHostname(hostname string, fs vfs.FS) error {
	return fs.WriteFile("/etc/hostname", []byte(hostname+"\n"), 0644)
}

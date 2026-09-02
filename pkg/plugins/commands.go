package plugins

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs/v5"
)

func Commands(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	var errs error
	for _, cmd := range s.Commands {
		out, err := runCommand(l, fs, console, templateSysData(l, cmd))
		if err != nil {
			if strings.TrimSpace(out) != "" {
				errs = multierror.Append(errs, fmt.Errorf("%w\ncommand output:\n%s", err, out))
			} else {
				errs = multierror.Append(errs, err)
			}
			continue
		}
		if strings.TrimSpace(out) != "" {
			l.Debug(fmt.Sprintf("Command output: %s", out))
		} else {
			l.Debugf("Empty command output")
		}

	}
	return errs
}

// runCommand runs one command through the console.
//
// A command that opens with a shebang is a script, and the interpreter that
// line names is the one that has to read it. The console runs commands under
// `sh -c`, where the shebang is only a comment and sh runs the body itself.
// Write such a command out as a file instead, so the kernel reads the first
// line and starts the interpreter the author asked for.
func runCommand(l logger.Interface, fs vfs.FS, console Console, command string) (string, error) {
	if !strings.HasPrefix(command, "#!") {
		return console.Run(command)
	}

	path, err := writeScript(fs, command)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := fs.Remove(path); err != nil {
			l.Debugf("could not remove %s: %s", path, err)
		}
	}()

	// The console runs processes on the host, so it needs the path as the
	// host sees it.
	hostPath, err := fs.RawPath(path)
	if err != nil {
		return "", fmt.Errorf("could not resolve %s: %w", path, err)
	}

	l.Debugf("command carries a shebang, running it as %s", hostPath)
	return console.Run(hostPath)
}

// writeScript stores script in a private executable file and returns its path.
func writeScript(fs vfs.FS, script string) (string, error) {
	name := make([]byte, 8)
	if _, err := rand.Read(name); err != nil {
		return "", fmt.Errorf("could not name a script file: %w", err)
	}
	path := filepath.Join(os.TempDir(), "yip-command-"+hex.EncodeToString(name))

	// O_EXCL so an existing path, a planted symlink included, fails here
	// instead of becoming the file we write through.
	f, err := fs.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0700)
	if err != nil {
		return "", fmt.Errorf("could not create %s: %w", path, err)
	}

	if _, err := f.WriteString(script); err != nil {
		f.Close()
		_ = fs.Remove(path)
		return "", fmt.Errorf("could not write %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		_ = fs.Remove(path)
		return "", fmt.Errorf("could not close %s: %w", path, err)
	}

	// The mode above passes through the umask. Set it outright, the kernel
	// refuses to execute a file that is not marked executable.
	if err := fs.Chmod(path, 0700); err != nil {
		_ = fs.Remove(path)
		return "", fmt.Errorf("could not make %s executable: %w", path, err)
	}

	return path, nil
}

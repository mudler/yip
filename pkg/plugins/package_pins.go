package plugins

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs/v4"
)

// PackagePins applies a best-effort "pin/version lock" policy before package installs.
//
// Supports:
// - APT (Debian/Ubuntu): /etc/apt/preferences.d/99-yip-pins with Pin-Priority: 1001
// - DNF (Fedora/RHEL/etc.): /etc/dnf/plugins/versionlock.conf + locklist file
//   - resolves strict NEVRA via `dnf repoquery` when possible; falls back to raw patterns if resolution fails
//
// - SUSE (openSUSE/SLE): zypper addlock (tries name=version; falls back to locking name)
// - Alpine: updates /etc/apk/world to include name=version constraints (idempotent)
//
// Other installers: warn + no-op (extend later).
func PackagePins(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	if len(s.PackagePins) == 0 {
		return nil
	}

	installer := identifyInstaller(fs)

	// normalize + sort for deterministic output
	keys := make([]string, 0, len(s.PackagePins))
	for k := range s.PackagePins {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	switch installer {
	case APTInstaller:
		return applyAptPins(l, fs, keys, s.PackagePins)
	case DNFInstaller:
		return applyDnfVersionlock(l, fs, console, keys, s.PackagePins)
	case SUSEInstaller:
		return applyZypperLocks(l, console, keys, s.PackagePins)
	case AlpineInstaller:
		return applyApkWorldPins(l, fs, keys, s.PackagePins)
	case PacmanInstaller:
		l.Warnf("package_pins: pacman does not support native version pinning; skipping")
		return nil
	default:
		l.Warnf("package_pins: installer '%s' not supported yet; skipping", installer.String())
		return nil
	}
}

func applyAptPins(l logger.Interface, fs vfs.FS, keys []string, pins map[string]string) error {
	const dir = "/etc/apt/preferences.d"
	const file = "/etc/apt/preferences.d/99-yip-pins"

	if err := vfs.MkdirAll(fs, dir, 0755); err != nil {
		return err
	}

	var b bytes.Buffer
	b.WriteString("# Managed by yip (package_pins). DO NOT EDIT.\n")
	b.WriteString("# This file is rewritten on each yip run.\n\n")

	for _, name := range keys {
		ver := strings.TrimSpace(pins[name])
		if ver == "" {
			l.Warnf("package_pins: empty version for '%s' (skipping)", name)
			continue
		}

		// Force candidate selection + allow downgrade (1001)
		fmt.Fprintf(&b, "Package: %s\n", name)
		fmt.Fprintf(&b, "Pin: version %s\n", ver)
		b.WriteString("Pin-Priority: 1001\n\n")
	}

	return writeDirect(fs, file, b.String(), 0644)
}

func applyDnfVersionlock(l logger.Interface, fs vfs.FS, console Console, keys []string, pins map[string]string) error {
	// DNF versionlock plugin uses a config file + locklist.
	// We write both in a deterministic way. DNF will enforce on subsequent dnf operations.
	const pluginDir = "/etc/dnf/plugins"
	const confPath = "/etc/dnf/plugins/versionlock.conf"
	const lockListPath = "/etc/dnf/plugins/versionlock.list"

	if err := vfs.MkdirAll(fs, pluginDir, 0755); err != nil {
		return err
	}

	conf := strings.Join([]string{
		"# Managed by yip (package_pins). DO NOT EDIT.",
		"[main]",
		"enabled=1",
		fmt.Sprintf("locklist=%s", lockListPath),
		"",
	}, "\n")

	if err := writeDirect(fs, confPath, conf, 0644); err != nil {
		return err
	}

	var b bytes.Buffer
	b.WriteString("# Managed by yip (package_pins). DO NOT EDIT.\n")
	b.WriteString("# Prefer strict NEVRA locks; fallback to raw patterns if resolution fails.\n\n")

	for _, name := range keys {
		ver := strings.TrimSpace(pins[name])
		if ver == "" {
			l.Warnf("package_pins: empty version for '%s' (skipping)", name)
			continue
		}

		nevra, err := resolveDnfNEVRA(l, console, name, ver)
		if err == nil && strings.TrimSpace(nevra) != "" {
			fmt.Fprintf(&b, "%s\n", strings.TrimSpace(nevra))
			continue
		}

		l.Warnf("package_pins(dnf): could not resolve NEVRA for %s=%s, falling back to raw lock pattern", name, ver)
		// raw-ish fallback; versionlock will treat it as a pattern in many setups
		fmt.Fprintf(&b, "%s-%s*\n", name, ver)
	}

	return writeDirect(fs, lockListPath, b.String(), 0644)
}

// resolveDnfNEVRA attempts to find a matching NEVRA line via dnf repoquery.
// Returns first match (best effort).
// NEVRA (Name-Epoch-Version-Release-Arch) is the canonical RPM identifier:
//
//	name-epoch:version-release.arch
//
// Example:
//
//	nginx-1:1.24.0-3.el9.x86_64
//
// It uniquely identifies a specific RPM build. Used for strict version locking in DNF.
func resolveDnfNEVRA(l logger.Interface, console Console, name, ver string) (string, error) {
	// Queryformat prints: name-epoch:version-release.arch
	// Match by "name-version*" (release varies), then take first result line.
	//
	// We wrap queryformat in single quotes for the shell.
	qf := "'%{name}-%{epoch}:%{version}-%{release}.%{arch}'"
	spec := fmt.Sprintf("%s-%s*", name, ver)

	cmd := fmt.Sprintf("dnf -q repoquery --qf %s %s", qf, shellEscape(spec))
	out, err := console.Run(templateSysData(l, cmd))
	if err != nil {
		return "", err
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		if strings.HasPrefix(ln, name+"-") {
			return ln, nil
		}
	}
	return "", errors.New("no repoquery matches")
}

func applyZypperLocks(l logger.Interface, console Console, keys []string, pins map[string]string) error {
	for _, name := range keys {
		ver := strings.TrimSpace(pins[name])
		if ver == "" {
			l.Warnf("package_pins: empty version for '%s' (skipping)", name)
			continue
		}

		// Prefer locking capability "name=version" if zypper accepts it.
		cap := fmt.Sprintf("%s=%s", name, ver)
		cmd := fmt.Sprintf("zypper --non-interactive addlock %s", shellEscape(cap))
		out, err := console.Run(templateSysData(l, cmd))
		if err == nil {
			if strings.TrimSpace(out) != "" {
				l.Debugf("package_pins(zypper): %s", strings.TrimSpace(out))
			}
			continue
		}

		// Fallback: lock only the name.
		l.Warnf("package_pins(zypper): failed to lock %s, falling back to locking package name only", cap)
		cmd2 := fmt.Sprintf("zypper --non-interactive addlock %s", shellEscape(name))
		out2, err2 := console.Run(templateSysData(l, cmd2))
		if err2 != nil {
			// Best-effort: don't hard-fail
			l.Warnf("package_pins(zypper): failed to lock %s: %v (output: %s)", name, err2, strings.TrimSpace(out2))
		}
	}
	return nil
}

func applyApkWorldPins(l logger.Interface, fs vfs.FS, keys []string, pins map[string]string) error {
	// Alpine pinning: manage /etc/apk/world exclusively.
	// We overwrite the file deterministically with only yip-managed pins (name=version).
	const worldPath = "/etc/apk/world"

	if err := vfs.MkdirAll(fs, "/etc/apk", 0755); err != nil {
		return err
	}

	var out bytes.Buffer
	out.WriteString("# Managed by yip (package_pins). DO NOT EDIT.\n")
	out.WriteString("# This file is rewritten on each yip run.\n")

	for _, name := range keys {
		ver := strings.TrimSpace(pins[name])
		if ver == "" {
			l.Warnf("package_pins: empty version for '%s' (skipping)", name)
			continue
		}
		out.WriteString(fmt.Sprintf("%s=%s\n", name, ver))
	}

	return writeDirect(fs, worldPath, out.String(), 0644)
}

func writeDirect(FS vfs.FS, path, content string, mode uint32) error {
	dir := filepath.Dir(path)
	if err := vfs.MkdirAll(FS, dir, 0755); err != nil {
		return err
	}
	f, err := FS.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		return err
	}
	// Best effort chmod: some vfs backends ignore this (e.g. memfs), but we try anyway for those that support it.
	_ = FS.Chmod(path, fs.FileMode(mode))
	return nil
}

// shellEscape wraps a string in single quotes and escapes embedded single quotes.
// Good enough for our usage (capabilities and specs).
func shellEscape(s string) string {
	// ' -> '"'"'
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

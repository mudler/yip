package plugins

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs/v4"
)

type Installer string

const (
	APTInstaller     Installer = "apt-get"
	DNFInstaller     Installer = "dnf"
	PacmanInstaller  Installer = "pacman"
	SUSEInstaller    Installer = "zypper"
	AlpineInstaller  Installer = "apk"
	UnknownInstaller Installer = "unknown"
)

func (d Installer) String() string {
	return string(d)
}

type Distro string

const (
	Debian             Distro = "debian"
	Ubuntu             Distro = "ubuntu"
	RedHat             Distro = "rhel"
	CentOS             Distro = "centos"
	RockyLinux         Distro = "rocky"
	AlmaLinux          Distro = "almalinux"
	Fedora             Distro = "fedora"
	Arch               Distro = "arch"
	Alpine             Distro = "alpine"
	OpenSUSELeap       Distro = "opensuse-leap"
	OpenSUSETumbleweed Distro = "opensuse-tumbleweed"
	SUSE               Distro = "suse"
	OpenEuler          Distro = "openEuler"
)

// Packages runs the package manager to try to install/remove/upgrade/refresh packages
// It will try to identify the package manager based on the distro
// If it can't identify the package manager, it will return an error
// Order is Refresh -> Upgrade -> Install -> Remove
func Packages(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	// Don't do anything if empty
	if len(s.Packages.Remove) == 0 && len(s.Packages.Install) == 0 && s.Packages.Refresh == false {
		return nil
	}

	var installArgs, updateArgs, removeArgs, refreshArgs []string

	cmd := identifyInstaller(fs)

	switch cmd {
	case APTInstaller:
		// Needed so it doesn't ask for user input
		_ = os.Setenv("DEBIAN_FRONTEND", "noninteractive")
		defer func() {
			_ = os.Unsetenv("DEBIAN_FRONTEND")
		}()
		refreshArgs = []string{"-y", "update"}
		updateArgs = []string{"-y", "upgrade"}
		installArgs = []string{"-y", "--no-install-recommends", "install"}
		removeArgs = []string{"-y", "remove"}
	case AlpineInstaller:
		refreshArgs = []string{"update"}
		updateArgs = []string{"upgrade", "--no-cache"}
		installArgs = []string{"add", "--no-cache"}
		removeArgs = []string{"del", "--no-cache"}
	case DNFInstaller:
		refreshArgs = []string{"makecache"}
		updateArgs = []string{"update", "-y"}
		installArgs = []string{"install", "-y", "--setopt=install_weak_deps=False"}
		removeArgs = []string{"remove", "-y"}
	case SUSEInstaller:
		refreshArgs = []string{"refresh"}
		updateArgs = []string{"update", "-y"}
		installArgs = []string{"install", "-y", "--no-recommends"}
		removeArgs = []string{"remove", "-y"}
	case PacmanInstaller:
		refreshArgs = []string{"-Sy", "--noconfirm"}
		updateArgs = []string{"-Syu", "--noconfirm"}
		installArgs = []string{"-S", "--noconfirm"}
		removeArgs = []string{"-R", "--noconfirm"}
	default:
		l.Errorf("Unknown installer")
		return errors.New("unknown package manager")
	}
	// Run update databases/repos
	if s.Packages.Refresh {
		l.Debugf("Running refresh")
		out, err := console.Run(templateSysData(l, strings.Join(append([]string{cmd.String()}, refreshArgs...), " ")))
		if err != nil {
			l.Debug(fmt.Sprintf("Command output: %s", out))
			return err
		}
		if strings.TrimSpace(out) != "" {
			l.Debug(fmt.Sprintf("Command output: %s", out))
		} else {
			l.Debugf("Empty command output")
		}
	}

	// Upgrade packages
	if s.Packages.Upgrade {
		l.Debugf("Running upgrade")
		out, err := console.Run(templateSysData(l, strings.Join(append([]string{cmd.String()}, updateArgs...), " ")))
		if err != nil {
			l.Debug(fmt.Sprintf("Command output: %s", out))
			return err
		}
		if strings.TrimSpace(out) != "" {
			l.Debug(fmt.Sprintf("Command output: %s", out))
		} else {
			l.Debugf("Empty command output")
		}
	}

	if s.Packages.Install != nil {
		// Run install
		installArgs = append(installArgs, s.Packages.Install...)
		l.Debugf("Running install")
		out, err := console.Run(templateSysData(l, strings.Join(append([]string{cmd.String()}, installArgs...), " ")))
		if err != nil {
			l.Debug(fmt.Sprintf("Command output: %s", out))
			return err
		}
		if strings.TrimSpace(out) != "" {
			l.Debug(fmt.Sprintf("Command output: %s", out))
		} else {
			l.Debugf("Empty command output")
		}
	}

	if s.Packages.Remove != nil {
		// Run remove
		removeArgs = append(removeArgs, s.Packages.Remove...)
		l.Debugf("Running remove")
		out, err := console.Run(templateSysData(l, strings.Join(append([]string{cmd.String()}, removeArgs...), " ")))
		if err != nil {
			l.Debug(fmt.Sprintf("Command output: %s", out))
			return err
		}
		if strings.TrimSpace(out) != "" {
			l.Debug(fmt.Sprintf("Command output: %s", out))
		} else {
			l.Debugf("Empty command output")
		}
	}

	return nil
}

// identifyInstaller returns the package manager based on the distro
func identifyInstaller(fsys vfs.FS) Installer {
	identifiedInstaller := UnknownInstaller
	file, err := fsys.Open("/etc/os-release")
	if err != nil {
		return identifiedInstaller
	}
	defer func(file fs.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)
	val, err := godotenv.Parse(file)
	if err != nil {
		return identifiedInstaller
	}
	switch Distro(val["ID"]) {
	case Debian, Ubuntu:
		identifiedInstaller = APTInstaller
	case Fedora, RockyLinux, AlmaLinux, RedHat, CentOS, OpenEuler:
		identifiedInstaller = DNFInstaller
	case Arch:
		identifiedInstaller = PacmanInstaller
	case Alpine:
		identifiedInstaller = AlpineInstaller
	case OpenSUSELeap, OpenSUSETumbleweed, SUSE:
		identifiedInstaller = SUSEInstaller
	}

	// Catch-all for distros which determine a common ID_LIKE
	if identifiedInstaller == UnknownInstaller {
		switch Distro(val["ID_LIKE"]) {
		case SUSE:
			identifiedInstaller = SUSEInstaller
		}
	}

	return identifiedInstaller
}

package plugins

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs/v5"
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

// packageAttempts is how many times a package manager command is run before the
// step gives up, and packageRetryWait is how long to wait before the second
// attempt (it triples for each further one). They are variables so a test can
// shorten the wait.
var (
	packageAttempts  = 3
	packageRetryWait = 5 * time.Second
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
	OracleLinux        Distro = "ol"
	AlmaLinux          Distro = "almalinux"
	Fedora             Distro = "fedora"
	Arch               Distro = "arch"
	Alpine             Distro = "alpine"
	OpenSUSELeap       Distro = "opensuse-leap"
	OpenSUSETumbleweed Distro = "opensuse-tumbleweed"
	SUSE               Distro = "suse"
	OpenEuler          Distro = "openEuler"
)

// runPackageCommand runs one package manager command and retries it when it
// fails. A package manager reports a mirror it could not reach as a plain
// command failure, so without a retry every caller is only as reliable as the
// worst mirror the metalink hands out. Re-running the command resolves the
// mirror list again, which is what makes the retry worth anything.
func runPackageCommand(l logger.Interface, console Console, step, command string) error {
	var out string
	var err error

	wait := packageRetryWait
	for attempt := 1; attempt <= packageAttempts; attempt++ {
		l.Debugf("Running %s (attempt %d of %d)", step, attempt, packageAttempts)
		out, err = console.Run(templateSysData(l, command))
		if strings.TrimSpace(out) != "" {
			l.Debug(fmt.Sprintf("Command output: %s", out))
		} else {
			l.Debugf("Empty command output")
		}
		if err == nil {
			return nil
		}
		if attempt < packageAttempts {
			l.Warnf("Package %s failed: %s. Retrying in %s", step, err.Error(), wait)
			time.Sleep(wait)
			wait *= 3
		}
	}

	l.Errorf("Package %s failed after %d attempts: %s", step, packageAttempts, err.Error())
	return err
}

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
		command := strings.Join(append([]string{cmd.String()}, refreshArgs...), " ")
		if err := runPackageCommand(l, console, "refresh", command); err != nil {
			return err
		}
	}

	// Upgrade packages
	if s.Packages.Upgrade {
		command := strings.Join(append([]string{cmd.String()}, updateArgs...), " ")
		if err := runPackageCommand(l, console, "upgrade", command); err != nil {
			return err
		}
	}

	if s.Packages.Install != nil {
		installArgs = append(installArgs, s.Packages.Install...)
		command := strings.Join(append([]string{cmd.String()}, installArgs...), " ")
		if err := runPackageCommand(l, console, "install", command); err != nil {
			return err
		}
	}

	if s.Packages.Remove != nil {
		removeArgs = append(removeArgs, s.Packages.Remove...)
		command := strings.Join(append([]string{cmd.String()}, removeArgs...), " ")
		if err := runPackageCommand(l, console, "remove", command); err != nil {
			return err
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
	case Fedora, RockyLinux, OracleLinux, AlmaLinux, RedHat, CentOS, OpenEuler:
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

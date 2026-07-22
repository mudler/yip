package plugins

import (
	"fmt"
	"os"
	osuser "os/user"
	"sort"
	"strconv"
	"syscall"

	"github.com/mauromorales/xpasswd/pkg/users"
	"github.com/pkg/errors"

	"github.com/hashicorp/go-multierror"
	"github.com/joho/godotenv"
	entities "github.com/mudler/entities/pkg/entities"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs/v5"
)

// shadowWithPreservedAging builds a Shadow entry for username/password while
// preserving the password-aging fields (minimum age, maximum age, warning days,
// inactive days, account expiration and the reserved field) from any existing
// /etc/shadow entry. entities.Shadow.Apply rewrites the whole row, so without
// this any aging policy set via chage or shipped in the base image would be
// wiped every time a yip stage updates the user's password.
func shadowWithPreservedAging(etcshadow, username, password string) *entities.Shadow {
	userShadow := &entities.Shadow{
		Username:    username,
		Password:    password,
		LastChanged: "now",
	}

	current, err := entities.ParseShadow(etcshadow)
	if err != nil {
		// No existing /etc/shadow (or it can't be parsed): nothing to preserve,
		// this is a brand-new entry.
		return userShadow
	}

	if existing, ok := current[username]; ok {
		userShadow.MinimumChanged = existing.MinimumChanged
		userShadow.MaximumChanged = existing.MaximumChanged
		userShadow.Warn = existing.Warn
		userShadow.Inactive = existing.Inactive
		userShadow.Expire = existing.Expire
		userShadow.Reserved = existing.Reserved
	}

	return userShadow
}

// HomeDirResolver resolves the numeric uid/gid owning a user's home directory.
// It exists so that the "reuse the id of the existing home directory" logic can
// be mocked in tests (setting real file ownership requires root), following the
// same pattern as DefaultFilesystemDetector/DefaultGrowFsToMax in this package.
type HomeDirResolver interface {
	// Resolve returns the uid and gid owning path and whether it exists.
	Resolve(fs vfs.FS, path string) (uid int, gid int, ok bool)
}

type realHomeDirResolver struct{}

func (realHomeDirResolver) Resolve(_ vfs.FS, path string) (int, int, bool) {
	// NOTE: we intentionally stat the path directly (not through fs.RawPath) to
	// keep the historical production behaviour. In production the filesystem is
	// vfs.OSFS where RawPath is the identity, so this is equivalent.
	info, err := os.Stat(path)
	if err != nil {
		return 0, 0, false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return int(stat.Uid), int(stat.Gid), true
}

// DefaultHomeDirResolver is the resolver used to look up the owner of existing
// home directories. Tests override it to simulate persistent home directories.
var DefaultHomeDirResolver HomeDirResolver = realHomeDirResolver{}

// userDefaults reads /etc/default/useradd (if present) and returns the default
// HOME base directory and SHELL, applying the same fallbacks as createUser.
func userDefaults(fs vfs.FS) map[string]string {
	usrDefaults := map[string]string{}
	if useradd, err := fs.RawPath("/etc/default/useradd"); err == nil {
		if _, err := os.Stat(useradd); err == nil {
			if d, err := godotenv.Read(useradd); err == nil {
				usrDefaults = d
			}
		}
	}
	if usrDefaults["SHELL"] == "" {
		usrDefaults["SHELL"] = "/bin/sh"
	}
	if usrDefaults["HOME"] == "" {
		usrDefaults["HOME"] = "/home"
	}
	return usrDefaults
}

// resolveHomedir returns the home directory that createUser would use for u.
func resolveHomedir(fs vfs.FS, u schema.User) string {
	if u.Homedir != "" {
		return u.Homedir
	}
	return fmt.Sprintf("%s/%s", userDefaults(fs)["HOME"], u.Name)
}

// hasDeterministicUID reports whether the uid of u is already fixed and does not
// need to be generated from the pool of free ids. This is the case when the uid
// is set explicitly, when the user already exists in /etc/passwd, or when the
// user's home directory already exists on disk (so its owner uid must be
// reused). Users with a deterministic uid must be (re)created BEFORE users that
// need a generated uid, otherwise a brand new user could steal the id that
// belongs to an existing (but not yet processed) user's home directory,
// swapping their uids on the next boot. See:
// https://github.com/kairos-io/kairos/issues/2949
func hasDeterministicUID(fs vfs.FS, u schema.User) bool {
	if u.UID != "" {
		return true
	}
	if etcpasswd, err := fs.RawPath("/etc/passwd"); err == nil {
		list := users.NewUserList()
		list.SetPath(etcpasswd)
		if err := list.Load(); err == nil && list.Get(u.Name) != nil {
			return true
		}
	}
	_, _, ok := DefaultHomeDirResolver.Resolve(fs, resolveHomedir(fs, u))
	return ok
}

// groupGIDInUse reports whether gid is already assigned to a group in etcgroup.
func groupGIDInUse(etcgroup string, gid int) bool {
	groups, err := entities.ParseGroup(etcgroup)
	if err != nil {
		return false
	}
	for _, g := range groups {
		if g.Gid != nil && *g.Gid == gid {
			return true
		}
	}
	return false
}

func createUser(fs vfs.FS, u schema.User, console Console) error {
	pass := u.PasswordHash
	if u.LockPasswd {
		pass = "!"
	}

	etcgroup, err := fs.RawPath("/etc/group")
	if err != nil {
		return errors.Wrap(err, "getting rawpath for /etc/group")
	}

	etcshadow, err := fs.RawPath("/etc/shadow")
	if err != nil {
		return errors.Wrap(err, "getting rawpath for /etc/shadow")
	}

	etcpasswd, err := fs.RawPath("/etc/passwd")
	if err != nil {
		return errors.Wrap(err, "getting rawpath for /etc/passwd")
	}

	useradd, err := fs.RawPath("/etc/default/useradd")
	if err != nil {
		return errors.Wrap(err, "getting rawpath for /etc/default/useradd")
	}

	usrDefaults := map[string]string{}

	// Load default home and shell from `/etc/default/useradd`
	if _, err = os.Stat(useradd); err == nil {
		usrDefaults, err = godotenv.Read(useradd)
		if err != nil {
			return errors.Wrapf(err, "could not parse '%s'", useradd)
		}
	}

	// Set default home and shell if they are empty
	if usrDefaults["SHELL"] == "" {
		usrDefaults["SHELL"] = "/bin/sh"
	}
	if usrDefaults["HOME"] == "" {
		usrDefaults["HOME"] = "/home"
	}

	primaryGroup := u.Name

	gid := -1 // -1 instructs entities to find the next free id and assign it
	if u.PrimaryGroup != "" {
		// An explicit primary_group names an existing system group. Look up its
		// numeric gid and reuse it. Explicit u.GID is intentionally ignored in
		// this branch — rewriting a well-known group's gid (e.g. wheel, docker)
		// would silently break every file already owned by that gid, which is
		// almost never what the caller wants. If the intent is to create the
		// user's own group with a pinned gid, leave primary_group empty and set
		// gid instead.
		gr, err := osuser.LookupGroup(u.PrimaryGroup)
		if err != nil {
			return errors.Wrap(err, "could not resolve primary group of user")
		}
		gid, _ = strconv.Atoi(gr.Gid)
		primaryGroup = u.PrimaryGroup
	} else if u.GID != "" {
		// Explicit gid without an explicit primary_group: create the user's own
		// primary group (named after the user) with this exact gid. Same
		// semantics as an explicit uid — wins over home-directory gid reuse.
		gid, err = strconv.Atoi(u.GID)
		if err != nil {
			return errors.Wrap(err, "invalid gid defined")
		}
	} else if _, hgid, ok := DefaultHomeDirResolver.Resolve(fs, resolveHomedir(fs, u)); ok && !groupGIDInUse(etcgroup, hgid) {
		// The user is being (re)created but its home directory already exists
		// (e.g. an immutable OS that regenerates /etc/{passwd,group} on every
		// boot while /home is persisted). Reuse the gid that owns the home
		// directory so file ownership stays consistent across boots instead of
		// letting entities pick a new (possibly different) free gid.
		// https://github.com/kairos-io/kairos/issues/2949
		gid = hgid
	}

	updateGroup := entities.Group{
		Name:     primaryGroup,
		Password: "x",
		Gid:      &gid,
		Users:    u.Name,
	}
	err = updateGroup.Apply(etcgroup, false)
	if err != nil {
		return errors.Wrap(err, "creating the user's group")
	}

	// reload the group to get the generated GID
	groups, _ := entities.ParseGroup(etcgroup)
	for name, group := range groups {
		if name == updateGroup.Name {
			updateGroup = group
			gid = *group.Gid
			break
		}
	}

	if u.Homedir == "" {
		u.Homedir = fmt.Sprintf("%s/%s", usrDefaults["HOME"], u.Name)
	}

	uid := -1

	// If UID is specified just put it there. No matter whats in the system or the collisions. Good luck.
	if u.UID != "" {
		// User defined-uid
		uid, err = strconv.Atoi(u.UID)
		if err != nil {
			return errors.Wrap(err, "invalid uid defined")
		}
	} else {
		// Try to get the existing UID in the system
		list := users.NewUserList()
		list.SetPath(etcpasswd)
		list.Load()
		user := list.Get(u.Name)
		if user != nil {
			uid, err = user.UID()
			if err != nil {
				return errors.Wrap(err, "could not get user id")
			}
		} else if homeUID, _, ok := DefaultHomeDirResolver.Resolve(fs, u.Homedir); ok {
			// Try to see if the user was created previously with a given UID by
			// checking the owner of the existing home directory and reuse it.
			uid = homeUID
		} else {
			// Now generate one if we havent been able to pick the existing one
			// https://systemd.io/UIDS-GIDS/#special-distribution-uid-ranges
			uid, err = list.GenerateUIDInRange(entities.HumanIDMin, entities.HumanIDMax)
			if err != nil {
				return errors.Wrap(err, "no available uid")
			}
		}
	}

	if uid == -1 {
		return errors.New("could not set uid for user")
	}

	if u.Shell == "" {
		u.Shell = usrDefaults["SHELL"]
	}

	userInfo := &entities.UserPasswd{
		Username: u.Name,
		Password: "x",
		Info:     u.GECOS,
		Homedir:  u.Homedir,
		Gid:      gid,
		Shell:    u.Shell,
		Uid:      uid,
	}

	if err := userInfo.Apply(etcpasswd, false); err != nil {
		return err
	}

	userShadow := shadowWithPreservedAging(etcshadow, u.Name, pass)
	if err := userShadow.Apply(etcshadow, false); err != nil {
		return err
	}

	if !u.NoCreateHome {
		homedir, err := fs.RawPath(u.Homedir)
		if err != nil {
			return errors.Wrap(err, "getting rawpath for homedir")
		}
		os.MkdirAll(homedir, 0755)
		os.Chown(homedir, uid, gid)
	}

	groups, _ = entities.ParseGroup(etcgroup)
	for name, group := range groups {
		for _, w := range u.Groups {
			if w == name {
				group.Users = u.Name
				group.Apply(etcgroup, false)
			}
		}
	}

	return nil
}

func setUserPass(fs vfs.FS, username, password string) error {
	etcshadow, err := fs.RawPath("/etc/shadow")
	if err != nil {
		return errors.Wrap(err, "getting rawpath for /etc/shadow")
	}
	userShadow := shadowWithPreservedAging(etcshadow, username, password)
	if err := userShadow.Apply(etcshadow, false); err != nil {
		return err
	}
	return nil
}

func User(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	var errs error

	// Order users so they get the same UID on each run
	names := make([]string, 0, len(s.Users))

	for k := range s.Users {
		names = append(names, k)
	}
	sort.Strings(names)

	// Split users into two groups, preserving alphabetical order within each:
	// those whose uid/gid is already deterministic (explicit uid or gid, already
	// present in /etc/passwd, or with an existing home directory) and those
	// that need a generated id. Deterministic users must be processed first so
	// that a brand new user cannot grab the id that belongs to an existing
	// user's home directory before that user is (re)created, which would swap
	// their ids and corrupt file ownership across boots.
	// https://github.com/kairos-io/kairos/issues/2949
	deterministic := make([]string, 0, len(names))
	generated := make([]string, 0, len(names))
	for _, k := range names {
		u := s.Users[k]
		u.Name = k
		if hasDeterministicUID(fs, u) {
			deterministic = append(deterministic, k)
		} else {
			generated = append(generated, k)
		}
	}
	ordered := append(deterministic, generated...)

	for _, k := range ordered {
		r := s.Users[k]
		r.Name = k
		if !r.Exists() {
			if err := createUser(fs, r, console); err != nil {
				errs = multierror.Append(errs, err)
			}
		} else if r.PasswordHash != "" {
			if err := setUserPass(fs, r.Name, r.PasswordHash); err != nil {
				return err
			}
		}

		if len(s.Users[k].SSHAuthorizedKeys) > 0 {
			SSH(l, schema.Stage{SSHKeys: map[string][]string{r.Name: r.SSHAuthorizedKeys}}, fs, console)
		}

	}
	return errs
}

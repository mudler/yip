package plugins

import (
	"os"
	osuser "os/user"
	"sort"
	"strconv"

	"github.com/pkg/errors"

	"github.com/hashicorp/go-multierror"
	entities "github.com/mudler/entities/pkg/entities"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs"
	passwd "github.com/willdonnelly/passwd"
)

func createUser(fs vfs.FS, u schema.User, console Console) error {

	userShadow := &entities.Shadow{
		Username:    u.Name,
		Password:    u.PasswordHash,
		LastChanged: "now",
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

	gid := 1000
	if u.PrimaryGroup != "" {
		gr, err := osuser.LookupGroup(u.PrimaryGroup)
		if err != nil {
			return errors.Wrap(err, "could not resolve primary group of user")
		}
		gid, _ = strconv.Atoi(gr.Gid)
	} else {
		// Create a new group after the user name
		all, _ := entities.ParseGroup(etcgroup)
		if len(all) != 0 {
			usedGids := []int{}
			for _, entry := range all {
				usedGids = append(usedGids, *entry.Gid)
			}
			sort.Ints(usedGids)
			if len(usedGids) == 0 {
				return errors.New("no new guid found")
			}
			gid = usedGids[len(usedGids)-1]
			gid++
		}

		newgroup := entities.Group{
			Name:     u.Name,
			Password: "x",
			Gid:      &gid,
			Users:    u.Name,
		}
		newgroup.Apply(etcgroup)
	}

	uid := 1000
	// find an available uid if there are others already
	all, _ := passwd.ParseFile(etcpasswd)
	if len(all) != 0 {
		usedUids := []int{}
		for _, entry := range all {
			uid, _ := strconv.Atoi(entry.Uid)
			usedUids = append(usedUids, uid)
		}
		sort.Ints(usedUids)
		if len(usedUids) == 0 {
			return errors.New("no new UID found")
		}
		uid = usedUids[len(usedUids)-1]
		uid++
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

	if err := userInfo.Apply(etcpasswd); err != nil {
		return err
	}

	if err := userShadow.Apply(etcshadow); err != nil {
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

	groups, _ := entities.ParseGroup(etcgroup)
	for name, group := range groups {
		for _, w := range u.Groups {
			if w == name {
				group.Users = group.Users + "," + u.Name
				group.Apply(etcgroup)
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
	userShadow := &entities.Shadow{
		Username:    username,
		Password:    password,
		LastChanged: "now",
	}
	if err := userShadow.Apply(etcshadow); err != nil {
		return err
	}
	return nil
}

func User(s schema.Stage, fs vfs.FS, console Console) error {
	var errs error

	for u, p := range s.Users {
		p.Name = u
		if !p.Exists() {
			if err := createUser(fs, p, console); err != nil {
				errs = multierror.Append(errs, err)
			}
		} else if p.PasswordHash != "" {
			if err := setUserPass(fs, p.Name, p.PasswordHash); err != nil {
				return err
			}
		}

		if len(p.SSHAuthorizedKeys) > 0 {
			SSH(schema.Stage{SSHKeys: map[string][]string{p.Name: p.SSHAuthorizedKeys}}, fs, console)
		}

	}
	return errs
}

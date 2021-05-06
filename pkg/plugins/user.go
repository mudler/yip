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

func createUser(u schema.User, console Console) error {

	userShadow := &entities.Shadow{
		Username:    u.Name,
		Password:    u.PasswordHash,
		LastChanged: "now",
	}

	gr, err := osuser.LookupGroup(u.PrimaryGroup)
	if err != nil {
		return errors.Wrap(err, "could not resolve primary group of user")
	}
	gid, _ := strconv.Atoi(gr.Gid)

	uid := 1000

	all, _ := passwd.ParseFile("/etc/passwd")
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

	if err := userInfo.Apply(""); err != nil {
		return err
	}

	if err := userShadow.Apply(""); err != nil {
		return err
	}
	if !u.NoCreateHome {
		os.MkdirAll(u.Homedir, 0755)
	}

	groups, _ := entities.ParseGroup("")
	for name, group := range groups {
		for _, w := range u.Groups {
			if w == name {
				group.Users = group.Users + "," + u.Name
				group.Apply("")
			}
		}
	}

	return nil
}

func User(s schema.Stage, fs vfs.FS, console Console) error {
	var errs error

	for u, p := range s.Users {
		p.Name = u
		if !p.Exists() {
			if err := createUser(p, console); err != nil {
				errs = multierror.Append(errs, err)
			}
		}
		/* 		else {
			if err := setUserPassword(u, p.PasswordHash, console); err != nil {
				errs = multierror.Append(errs, err)
			}
		} */
	}
	return errs
}

package runrunc

import (
	"path/filepath"

	"github.com/opencontainers/runc/libcontainer/user"
)

const (
	DefaultUID int = 0
	DefaultGID int = 0
)

func LookupUser(rootFsPath, userName string) (uid uint32, gid uint32, err error) {
	defaultUser := &user.ExecUser{Uid: DefaultUID, Gid: DefaultGID}
	passwdPath := filepath.Join(rootFsPath, "etc", "passwd")

	execUser, err := user.GetExecUserPath(userName, defaultUser, passwdPath, "")
	if err != nil {
		return 0, 0, err
	}

	return uint32(execUser.Uid), uint32(execUser.Gid), nil
}

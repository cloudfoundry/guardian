package runrunc

import (
	"path/filepath"

	"github.com/opencontainers/runc/libcontainer/user"
)

const (
	DefaultUID  int    = 0
	DefaultGID  int    = 0
	DefaultHome string = "/root"
)

func LookupUser(rootFsPath, userName string) (*ExecUser, error) {
	defaultUser := &user.ExecUser{Uid: DefaultUID, Gid: DefaultGID, Home: DefaultHome}
	passwdPath := filepath.Join(rootFsPath, "etc", "passwd")

	execUser, err := user.GetExecUserPath(userName, defaultUser, passwdPath, "")
	if err != nil {
		return nil, err
	}

	return &ExecUser{Uid: execUser.Uid, Gid: execUser.Gid, Home: execUser.Home, Sgids: execUser.Sgids}, nil
}

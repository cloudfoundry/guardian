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

func LookupUser(rootFsPath, userName string) (*user.ExecUser, error) {
	defaultUser := &user.ExecUser{Uid: DefaultUID, Gid: DefaultGID, Home: DefaultHome}
	passwdPath := filepath.Join(rootFsPath, "etc", "passwd")

	return user.GetExecUserPath(userName, defaultUser, passwdPath, "")
}

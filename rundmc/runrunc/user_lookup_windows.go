package runrunc

import (
	"fmt"

	"github.com/opencontainers/runc/libcontainer/user"
)

const (
	DefaultUID  int    = 0
	DefaultGID  int    = 0
	DefaultHome string = `C:\\Users\\ContainerAdministrator`
)

func LookupUser(rootFsPath, userName string) (*user.ExecUser, error) {
	user := &user.ExecUser{Uid: DefaultUID, Gid: DefaultGID, Home: DefaultHome}
	if userName != "" {
		user.Home = fmt.Sprintf("C:\\Users\\%s", userName)
	}
	return user, nil
}

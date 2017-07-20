package runrunc

import "fmt"

const (
	DefaultUID  int    = 0
	DefaultGID  int    = 0
	DefaultHome string = `C:\\Users\\ContainerAdministrator`
)

func LookupUser(rootFsPath, userName string) (*ExecUser, error) {
	user := &ExecUser{Uid: DefaultUID, Gid: DefaultGID, Home: DefaultHome}
	if userName != "" {
		user.Home = fmt.Sprintf("C:\\Users\\%s", userName)
	}
	return user, nil
}

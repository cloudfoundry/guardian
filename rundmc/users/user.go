package users

const (
	DefaultUID int = 0
	DefaultGID int = 0
)

type ExecUser struct {
	Uid   int
	Gid   int
	Sgids []int
	Home  string
}

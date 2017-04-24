package depot

import "os"

type OSChowner struct{}

func (*OSChowner) Chown(path string, uid, gid int) error {
	return os.Chown(path, uid, gid)
}

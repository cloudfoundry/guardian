// +build !cgo

// This file is a dummy structure to allow us to compile on a Mac with GOOS=linux

package quota

type Quota struct {
	Size   uint64
	BCount uint64
}

type Control struct {
}

func NewControl(basePath string) (*Control, error) {
	return nil, nil
}

func (q *Control) SetQuota(projectId uint32, targetPath string, quota Quota) error {
	return nil
}

func (q *Control) GetQuota(targetPath string, quota *Quota) error {
	return nil
}

func GetProjectID(targetPath string) (uint32, error) {
	return 0, nil
}

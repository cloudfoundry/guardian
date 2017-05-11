package locksmith

type FileSystem struct {
}

func NewFileSystem() *FileSystem {
	return &FileSystem{}
}

type fileSystemUnlocker struct {
}

func (u *fileSystemUnlocker) Unlock() error {
	return nil
}

func (l *FileSystem) Lock(path string) (Unlocker, error) {
	return &fileSystemUnlocker{}, nil
}

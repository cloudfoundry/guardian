package locksmith // import "code.cloudfoundry.org/grootfs/store/locksmith"

type FileSystem struct {
	storePath string
}

func NewFileSystem(storePath string) *FileSystem {
	return &FileSystem{
		storePath: storePath,
	}
}

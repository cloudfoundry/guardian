package rundmc

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

// a depot which stores containers as subdirs of a depot directory
type DirectoryDepot struct {
	Dir string
}

func (d *DirectoryDepot) Create(handle string) error {
	os.MkdirAll(filepath.Join(d.Dir, handle), 0700)
	return ioutil.WriteFile(filepath.Join(d.Dir, handle, "config.json"), nil, 0700)
}

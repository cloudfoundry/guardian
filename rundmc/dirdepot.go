package rundmc

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/cf-guardian/specs"
)

// a depot which stores containers as subdirs of a depot directory
type DirectoryDepot struct {
	Dir string
}

func (d *DirectoryDepot) Create(handle string) error {
	os.MkdirAll(filepath.Join(d.Dir, handle), 0700)
	b, err := json.Marshal(specs.Spec{
		Version: "pre-draft",
		Process: specs.Process{
			Args: []string{
				"/bin/echo", "Pid 1 Running",
			},
		},
	})
	if err != nil {
		panic(err)
	}
	return ioutil.WriteFile(filepath.Join(d.Dir, handle, "config.json"), b, 0700)
}

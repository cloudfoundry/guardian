package rundmc

import (
	"encoding/json"
	"io/ioutil"
	"os/exec"
	"path/filepath"

	"github.com/opencontainers/specs"
)

type Bundle struct {
	configJSON []byte
}

func (b Bundle) Create(path string) error {
	return ioutil.WriteFile(filepath.Join(path, "config.json"), b.configJSON, 0700)
}

func BundleForCmd(cmd *exec.Cmd) *Bundle {
	configJson, err := json.Marshal(specs.Spec{
		Version: "pre-draft",
		Process: specs.Process{
			Terminal: true,
			Args:     cmd.Args,
		},
	})

	if err != nil {
		panic(err) // can't happen
	}

	return &Bundle{
		configJSON: configJson,
	}
}

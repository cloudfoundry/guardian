package rundmc

import (
	"encoding/json"
	"io/ioutil"
	"os/exec"
	"path/filepath"

	"github.com/opencontainers/specs"
)

type Bundle struct {
	configJSON  []byte
	runtimeJSON []byte
}

func (b Bundle) Create(path string) error {
	err := ioutil.WriteFile(filepath.Join(path, "config.json"), b.configJSON, 0700)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath.Join(path, "runtime.json"), b.runtimeJSON, 0700)
	if err != nil {
		return err
	}
	return nil
}

type Mount struct {
	Name        string
	Source      string
	Destination string
	Type        string
	Options     []string
}

func BundleForCmd(cmd *exec.Cmd, mounts []Mount) *Bundle {
	configMounts := make([]specs.MountPoint, len(mounts))
	runtimeMounts := make(map[string]mount)
	for _, m := range mounts {
		configMounts = append(configMounts, specs.MountPoint{
			Name: m.Name,
			Path: m.Destination,
		})

		runtimeMounts[m.Name] = mount{
			Type:    m.Type,
			Source:  m.Source,
			Options: m.Options,
		}
	}

	configJson, err := json.Marshal(specs.Spec{
		Version: "0.1.0",
		Process: specs.Process{
			Args: cmd.Args,
		},
		Mounts: configMounts,
	})

	if err != nil {
		panic(err) // can't happen
	}

	runtimeJson, err := json.Marshal(RuntimeSpec{
		Mounts: runtimeMounts,
		Linux: Linux{
			Namespaces: []namespace{
				{Type: "mount"},
				{Type: "network"},
				{Type: "ipc"},
				{Type: "uts"},
				{Type: "pid"},
			},
			Resources: map[string]interface{}{
				"memory":        struct{}{},
				"cpu":           struct{}{},
				"pids":          struct{}{},
				"blockIO":       struct{}{},
				"hugepageLimit": struct{}{},
				"network":       struct{}{},
			},
		},
	})
	if err != nil {
		panic(err) // can't happen
	}

	return &Bundle{
		configJSON:  configJson,
		runtimeJSON: runtimeJson,
	}
}

type RuntimeSpec struct {
	Linux  Linux            `json:"linux"`
	Mounts map[string]mount `json:"mounts"`
}

type Linux struct {
	Namespaces []namespace            `json:"namespaces"`
	Resources  map[string]interface{} `json:"resources"`
}

type namespace struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

type mount struct {
	Type    string   `json:"type"`
	Source  string   `json:"source"`
	Options []string `json:"options"`
}

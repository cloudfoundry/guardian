package goci

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

type BndlLoader struct {
}

func (b *BndlLoader) Load(path string) (Bndl, error) {
	bundle := Bndl{}
	err := readJsonInto(filepath.Join(path, "config.json"), &bundle.Spec)
	if err != nil {
		return bundle, fmt.Errorf("Failed to load bundle: %s", err)
	}

	return bundle, nil
}

func (b Bndl) Save(path string) error {
	return save(b.Spec, filepath.Join(path, "config.json"))
}

func save(value interface{}, path string) error {
	w, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return fmt.Errorf("Failed to save bundle: %s", err)
	}

	return json.NewEncoder(w).Encode(value)
}

func readJsonInto(path string, object interface{}) error {
	runtimeContents, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(runtimeContents, object)
}

package guardiancmd

import (
	"fmt"
	"os"
	"path/filepath"
)

type DirFlag string

func (f *DirFlag) UnmarshalFlag(value string) error {
	stat, err := os.Stat(value)
	if err != nil {
		return err
	}

	if !stat.IsDir() {
		return fmt.Errorf("path '%s' is not a directory", value)
	}

	abs, err := filepath.Abs(value)
	if err != nil {
		return err
	}

	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return err
	}

	*f = DirFlag(resolved)

	return nil
}

func (f DirFlag) Path() string {
	return string(f)
}

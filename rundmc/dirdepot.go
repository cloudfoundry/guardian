package rundmc

import (
	"errors"
	"os"
	"path/filepath"
)

var ErrDoesNotExist = errors.New("does not exist")

//go:generate counterfeiter . BundleCreator
type BundleCreator interface {
	Create(path string) error
}

// a depot which stores containers as subdirs of a depot directory
type DirectoryDepot struct {
	Dir           string
	BundleCreator BundleCreator
}

func (d *DirectoryDepot) Create(handle string) error {
	path := filepath.Join(d.Dir, handle)
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}

	if err := d.BundleCreator.Create(filepath.Join(d.Dir, handle)); err != nil {
		os.RemoveAll(path)
		return err
	}

	return nil
}

func (d *DirectoryDepot) Lookup(handle string) (string, error) {
	if _, err := os.Stat(filepath.Join(d.Dir, handle)); err != nil {
		return "", ErrDoesNotExist
	}

	return filepath.Join(d.Dir, handle), nil
}

package cgroups

import (
	"errors"
	"os"
	"path/filepath"
)

//go:generate counterfeiter . Chowner
type Chowner interface {
	RecursiveChown(path string) error
}

type OSChowner struct {
	UID *int
	GID *int
}

func (c *OSChowner) RecursiveChown(path string) error {
	if (c.UID == nil) != (c.GID == nil) {
		return errors.New("either both UID and GID must be nil, or neither can be nil")
	}

	if c.UID == nil || c.GID == nil {
		return nil
	}

	return filepath.Walk(path, func(name string, info os.FileInfo, statErr error) error {
		if statErr != nil {
			return statErr
		}

		return os.Chown(name, *c.UID, *c.GID)
	})
}

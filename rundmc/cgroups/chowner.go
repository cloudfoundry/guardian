package cgroups

import (
	"errors"
	"os"
	"path/filepath"
)

//go:generate counterfeiter . Chowner
type Chowner interface {
	RecursiveChown(path string, uid, gid *int) error
}

type OSChowner struct{}

func (c *OSChowner) RecursiveChown(path string, uid, gid *int) error {
	if (uid == nil) != (gid == nil) {
		return errors.New("either both UID and GID must be nil, or neither can be nil")
	}

	if uid == nil || gid == nil {
		return nil
	}

	return filepath.Walk(path, func(name string, info os.FileInfo, statErr error) error {
		if statErr != nil {
			return statErr
		}

		return os.Chown(name, *uid, *gid)
	})
}

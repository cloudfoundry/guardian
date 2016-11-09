package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type RootFSConfigurer struct{}

func NewRootFSConfigurer() *RootFSConfigurer {
	return &RootFSConfigurer{}
}

func (r *RootFSConfigurer) Configure(rootFSPath string, baseImage specsv1.Image) error {
	_, err := os.Stat(rootFSPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("rootfs path does not exist: %s", err)
		}
	}

	for volumeRelPath, _ := range baseImage.Config.Volumes {
		volumeRelPath = filepath.Clean(volumeRelPath)
		volumePath := filepath.Join(rootFSPath, volumeRelPath)
		if !r.isRelativeTo(volumePath, rootFSPath) {
			return errors.New("volume path is outside of the rootfs")
		}

		stat, err := os.Stat(volumePath)
		if err == nil && !stat.IsDir() {
			return errors.New("a file with the requested volume path already exists")
		}

		if err := os.MkdirAll(volumePath, 0755); err != nil {
			return fmt.Errorf("making volume `%s`: %s", volumePath, err)
		}
	}

	return nil
}

func (r *RootFSConfigurer) isRelativeTo(volumePath, rootFSPath string) bool {
	return strings.HasPrefix(volumePath, rootFSPath)
}

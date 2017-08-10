package store

import (
	"os"
	"path/filepath"
	"strings"

	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
	errorspkg "github.com/pkg/errors"
)

type RootFSConfigurer struct{}

func NewRootFSConfigurer() *RootFSConfigurer {
	return &RootFSConfigurer{}
}

func (r *RootFSConfigurer) Configure(rootFSPath string, baseImage *specsv1.Image) error {
	if baseImage == nil {
		return nil
	}

	_, err := os.Stat(rootFSPath)
	if err != nil {
		if os.IsNotExist(err) {
			return errorspkg.Wrap(err, "rootfs path does not exist")
		}
	}

	for volumeRelPath, _ := range baseImage.Config.Volumes {
		volumeRelPath = filepath.Clean(volumeRelPath)
		volumePath := filepath.Join(rootFSPath, volumeRelPath)
		if !r.isRelativeTo(volumePath, rootFSPath) {
			return errorspkg.New("volume path is outside of the rootfs")
		}

		stat, err := os.Stat(volumePath)
		if err == nil && !stat.IsDir() {
			return errorspkg.New("a file with the requested volume path already exists")
		}

		if err := os.MkdirAll(volumePath, 0755); err != nil {
			return errorspkg.Wrapf(err, "making volume `%s`", volumePath)
		}
	}

	return nil
}

func (r *RootFSConfigurer) isRelativeTo(volumePath, rootFSPath string) bool {
	return strings.HasPrefix(volumePath, rootFSPath)
}

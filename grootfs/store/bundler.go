package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

//go:generate counterfeiter . SnapshotDriver

type SnapshotDriver interface {
	Snapshot(logger lager.Logger, fromPath, toPath string) error
	ApplyDiskLimit(logger lager.Logger, path string, diskLimit int64, exclusive bool) error
	FetchMetrics(logger lager.Logger, path string) (groot.VolumeMetrics, error)
	Destroy(logger lager.Logger, path string) error
}

type Bundler struct {
	snapshotDriver SnapshotDriver
	storePath      string
}

func NewBundler(snapshotDriver SnapshotDriver, storePath string) *Bundler {
	return &Bundler{
		snapshotDriver: snapshotDriver,
		storePath:      storePath,
	}
}

func (b *Bundler) Exists(id string) (bool, error) {
	bundlePath := path.Join(b.storePath, BUNDLES_DIR_NAME, id)
	if _, err := os.Stat(bundlePath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("checking if bundle `%s` exists: `%s`", id, err)
	}

	return true, nil
}

func (b *Bundler) Create(logger lager.Logger, id string, spec groot.BundleSpec) (groot.Bundle, error) {
	logger = logger.Session("making-bundle", lager.Data{"storePath": b.storePath, "id": id})
	logger.Info("start")
	defer logger.Info("end")

	var (
		bundle *Bundle
		err    error
	)

	defer func() {
		if err != nil && bundle != nil {
			log := logger.Session("create-failed-cleaning-up", lager.Data{
				"id":    id,
				"cause": err.Error(),
			})

			log.Info("start")
			defer log.Info("end")

			if err = b.snapshotDriver.Destroy(logger, bundle.RootFSPath()); err != nil {
				log.Error("destroying-rootfs-snapshot", err)
			}

			if err = b.deleteBundleDir(bundle); err != nil {
				log.Error("deleting-bundle-path", err)
			}
		}
	}()

	bundle = NewBundle(path.Join(b.storePath, BUNDLES_DIR_NAME, id))
	if err := os.Mkdir(bundle.Path(), 0700); err != nil {
		return nil, fmt.Errorf("making bundle path: %s", err)
	}

	if err = b.writeImageJSON(logger, bundle, spec.Image); err != nil {
		return nil, fmt.Errorf("creating image.json: %s", err)
	}

	if err = b.snapshotDriver.Snapshot(logger, spec.VolumePath, bundle.RootFSPath()); err != nil {
		return nil, fmt.Errorf("creating snapshot: %s", err)
	}

	if spec.DiskLimit > 0 {
		if err = b.snapshotDriver.ApplyDiskLimit(logger, bundle.RootFSPath(), spec.DiskLimit, spec.ExcludeImageFromQuota); err != nil {
			return nil, fmt.Errorf("applying disk limit: %s", err)
		}
	}

	return bundle, nil
}

func (b *Bundler) Destroy(logger lager.Logger, id string) error {
	logger = logger.Session("deleting-bundle", lager.Data{"storePath": b.storePath, "id": id})
	logger.Info("start")
	defer logger.Info("end")

	if ok, err := b.Exists(id); !ok {
		logger.Error("checking-bundle-path-failed", err)
		return fmt.Errorf("bundle not found: %s", id)
	}

	bundle := NewBundle(path.Join(b.storePath, BUNDLES_DIR_NAME, id))
	if err := b.snapshotDriver.Destroy(logger, bundle.RootFSPath()); err != nil {
		return fmt.Errorf("destroying snapshot: %s", err)
	}

	if err := b.deleteBundleDir(bundle); err != nil {
		return fmt.Errorf("deleting bundle path: %s", err)
	}

	return nil
}

func (b *Bundler) Metrics(logger lager.Logger, id string) (groot.VolumeMetrics, error) {
	logger = logger.Session("fetching-metrics", lager.Data{"id": id})
	logger.Info("start")
	defer logger.Info("end")

	bundle := NewBundle(path.Join(b.storePath, BUNDLES_DIR_NAME, id))
	return b.snapshotDriver.FetchMetrics(logger, bundle.RootFSPath())
}

func (b *Bundler) deleteBundleDir(bundle *Bundle) error {
	if err := os.RemoveAll(bundle.Path()); err != nil {
		return fmt.Errorf("deleting bundle path: %s", err)
	}

	return nil
}

var OF = os.OpenFile

func (b *Bundler) writeImageJSON(logger lager.Logger, bundle groot.Bundle, image specsv1.Image) error {
	logger = logger.Session("writing-image-json")
	logger.Info("start")
	defer logger.Info("end")

	imageJsonPath := filepath.Join(bundle.Path(), "image.json")
	imageJsonFile, err := OF(imageJsonPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}

	if err = json.NewEncoder(imageJsonFile).Encode(image); err != nil {
		return err
	}

	return nil
}

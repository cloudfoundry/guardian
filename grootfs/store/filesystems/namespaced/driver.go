package namespaced

import (
	"encoding/json"
	"os"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/sandbox"
	"code.cloudfoundry.org/grootfs/store/filesystems/loopback"
	"code.cloudfoundry.org/grootfs/store/filesystems/mount"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	"code.cloudfoundry.org/grootfs/store/filesystems/spec"
	"code.cloudfoundry.org/grootfs/store/image_manager"
	"code.cloudfoundry.org/lager/v3"
	"github.com/containers/storage/pkg/reexec"
	"github.com/pkg/errors"
)

//go:generate counterfeiter . internalDriver
type internalDriver interface {
	CreateVolume(logger lager.Logger, parentID string, id string) (string, error)
	DestroyVolume(logger lager.Logger, id string) error
	HandleOpaqueWhiteouts(logger lager.Logger, id string, opaqueWhiteouts []string) error
	MoveVolume(logger lager.Logger, from, to string) error
	VolumePath(logger lager.Logger, id string) (string, error)
	Volumes(logger lager.Logger) ([]string, error)
	WriteVolumeMeta(logger lager.Logger, id string, data base_image_puller.VolumeMeta) error
	MarkVolumeArtifacts(logger lager.Logger, id string) error

	CreateImage(logger lager.Logger, spec image_manager.ImageDriverSpec) (groot.MountInfo, error)
	DestroyImage(logger lager.Logger, path string) error
	FetchStats(logger lager.Logger, path string) (groot.VolumeStats, error)

	Marshal(logger lager.Logger) ([]byte, error)
}

type DirectIO interface {
	Configure(path string) error
}

type Driver struct {
	internalDriver
	reexecer          groot.SandboxReexecer
	shouldCloneUserNs bool
}

func New(driver internalDriver, reexecer groot.SandboxReexecer, shouldCloneUserNs bool) *Driver {
	return &Driver{
		internalDriver:    driver,
		reexecer:          reexecer,
		shouldCloneUserNs: shouldCloneUserNs,
	}
}

func init() {
	sandbox.Register("destroy-volume", func(logger lager.Logger, extraFiles []*os.File, args ...string) error {
		if len(os.Args) != 3 {
			return errors.New("drivers json or id not specified")
		}

		var driverSpec spec.DriverSpec
		if err := json.Unmarshal([]byte(os.Args[1]), &driverSpec); err != nil {
			return errors.Wrap(err, "unmarshaling driver spec")
		}

		driver, err := specToDriver(driverSpec)
		if err != nil {
			return errors.Wrap(err, "creating fsdriver")
		}

		volumeID := os.Args[2]
		if err := driver.DestroyVolume(logger, volumeID); err != nil {
			return errors.Wrap(err, "destroying volume")
		}
		return nil
	})

	sandbox.Register("destroy-image", func(logger lager.Logger, extraFiles []*os.File, args ...string) error {
		if len(os.Args) != 3 {
			return errors.New("drivers json or path not specified")
		}

		var driverSpec spec.DriverSpec
		if err := json.Unmarshal([]byte(os.Args[1]), &driverSpec); err != nil {
			return errors.Wrap(err, "unmashaling driver spec")
		}

		driver, err := specToDriver(driverSpec)
		if err != nil {
			return errors.Wrap(err, "creating fsdriver")
		}

		imagePath := os.Args[2]
		if err := driver.DestroyImage(logger, imagePath); err != nil {
			return errors.Wrap(err, "destroying image")
		}
		return nil
	})

	if reexec.Init() {
		// prevents infinite reexec loop
		// Details: https://medium.com/@teddyking/namespaces-in-go-reexec-3d1295b91af8
		os.Exit(0)
	}
}

func (d *Driver) DestroyVolume(logger lager.Logger, id string) error {
	if !d.shouldCloneUserNs {
		return d.internalDriver.DestroyVolume(logger, id)
	}

	logger = logger.Session("ns-destroy-volume")
	logger.Debug("starting")
	defer logger.Debug("ending")

	driverJSON, err := d.internalDriver.Marshal(logger)
	if err != nil {
		return errors.Wrap(err, "marshaling driver json")
	}

	if out, err := d.reexecer.Reexec("destroy-volume", groot.ReexecSpec{
		Args:        []string{string(driverJSON), id},
		CloneUserns: true,
	}); err != nil {
		return errors.Wrapf(err, "reexecing destroy volume: %s", string(out))
	}

	return nil
}

func (d *Driver) DestroyImage(logger lager.Logger, path string) error {
	if !d.shouldCloneUserNs {
		return d.internalDriver.DestroyImage(logger, path)
	}

	logger = logger.Session("ns-destroy-image")
	logger.Debug("starting")
	defer logger.Debug("ending")

	driverJSON, err := d.internalDriver.Marshal(logger)
	if err != nil {
		return errors.Wrapf(err, "marshaling driver json")
	}

	if out, err := d.reexecer.Reexec("destroy-image", groot.ReexecSpec{
		Args:        []string{string(driverJSON), path},
		CloneUserns: true,
	}); err != nil {
		return errors.Wrapf(err, "waiting for destroy image reexec: %s", string(out))
	}

	return nil
}

func specToDriver(spec spec.DriverSpec) (internalDriver, error) {
	switch spec.Type {
	case "overlay-xfs":
		return overlayxfs.NewDriver(
			spec.StorePath,
			spec.SuidBinaryPath,
			mount.RootlessUnmounter{},
			loopback.NewNoopDirectIO(),
		), nil
	default:
		return nil, errors.Errorf("invalid filesystem spec: %s not recognized", spec.Type)
	}
}

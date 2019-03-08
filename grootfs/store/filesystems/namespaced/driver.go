package namespaced

import (
	"encoding/json"
	"os"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/sandbox"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	"code.cloudfoundry.org/grootfs/store/filesystems/spec"
	"code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/lager"
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

	CreateImage(logger lager.Logger, spec image_cloner.ImageDriverSpec) (groot.MountInfo, error)
	DestroyImage(logger lager.Logger, path string) error
	FetchStats(logger lager.Logger, path string) (groot.VolumeStats, error)

	Marshal(logger lager.Logger) ([]byte, error)
}

type Driver struct {
	driver            internalDriver
	reexecer          groot.SandboxReexecer
	shouldCloneUserNs bool
}

func New(driver internalDriver, reexecer groot.SandboxReexecer, shouldCloneUserNs bool) *Driver {
	return &Driver{
		driver:            driver,
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

func (d *Driver) VolumePath(logger lager.Logger, id string) (string, error) {
	return d.driver.VolumePath(logger, id)
}

func (d *Driver) CreateVolume(logger lager.Logger, parentID string, id string) (string, error) {
	return d.driver.CreateVolume(logger, parentID, id)
}

func (d *Driver) DestroyVolume(logger lager.Logger, id string) error {
	if !d.shouldCloneUserNs {
		return d.driver.DestroyVolume(logger, id)
	}

	logger = logger.Session("ns-destroy-volume")
	logger.Debug("starting")
	defer logger.Debug("ending")

	driverJSON, err := d.driver.Marshal(logger)
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

func (d *Driver) Volumes(logger lager.Logger) ([]string, error) {
	return d.driver.Volumes(logger)
}

func (d *Driver) MoveVolume(logger lager.Logger, from, to string) error {
	return d.driver.MoveVolume(logger, from, to)
}

func (d *Driver) WriteVolumeMeta(logger lager.Logger, id string, data base_image_puller.VolumeMeta) error {
	return d.driver.WriteVolumeMeta(logger, id, data)
}

func (d *Driver) HandleOpaqueWhiteouts(logger lager.Logger, id string, opaqueWhiteouts []string) error {
	return d.driver.HandleOpaqueWhiteouts(logger, id, opaqueWhiteouts)
}

func (d *Driver) CreateImage(logger lager.Logger, spec image_cloner.ImageDriverSpec) (groot.MountInfo, error) {
	return d.driver.CreateImage(logger, spec)
}

func (d *Driver) DestroyImage(logger lager.Logger, path string) error {
	if !d.shouldCloneUserNs {
		return d.driver.DestroyImage(logger, path)
	}

	logger = logger.Session("ns-destroy-image")
	logger.Debug("starting")
	defer logger.Debug("ending")

	driverJSON, err := d.driver.Marshal(logger)
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

func (d *Driver) FetchStats(logger lager.Logger, path string) (groot.VolumeStats, error) {
	return d.driver.FetchStats(logger, path)
}

func (d *Driver) MarkVolumeArtifacts(logger lager.Logger, id string) error {
	return d.driver.MarkVolumeArtifacts(logger, id)
}

func specToDriver(spec spec.DriverSpec) (internalDriver, error) {
	switch spec.Type {
	case "overlay-xfs":
		return overlayxfs.NewDriver(
			spec.StorePath,
			spec.SuidBinaryPath), nil
	default:
		return nil, errors.Errorf("invalid filesystem spec: %s not recognized", spec.Type)
	}
}

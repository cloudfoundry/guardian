package namespaced

import (
	"bytes"
	"encoding/json"
	"os"
	"syscall"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store/filesystems/btrfs"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	"code.cloudfoundry.org/grootfs/store/filesystems/spec"
	"code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/lager"
	"github.com/containers/storage/pkg/reexec"
	"github.com/pkg/errors"
	"github.com/tscolari/lagregator"
	"github.com/urfave/cli"
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

	CreateImage(logger lager.Logger, spec image_cloner.ImageDriverSpec) (groot.MountInfo, error)
	DestroyImage(logger lager.Logger, path string) error
	FetchStats(logger lager.Logger, path string) (groot.VolumeStats, error)

	Marshal(logger lager.Logger) ([]byte, error)
}

type Driver struct {
	driver     internalDriver
	idMappings groot.IDMappings
	idMapper   unpacker.IDMapper
	runner     commandrunner.CommandRunner
}

func New(driver internalDriver, idMappings groot.IDMappings, idMapper unpacker.IDMapper, runner commandrunner.CommandRunner) *Driver {
	return &Driver{
		driver:     driver,
		idMappings: idMappings,
		idMapper:   idMapper,
		runner:     runner,
	}
}

func init() {
	if reexec.Init() {
		os.Exit(0)
	}

	reexec.Register("destroy-volume", func() {
		cli.ErrWriter = os.Stdout
		logger := lager.NewLogger("destroy-volume")
		logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.DEBUG))

		if len(os.Args) != 3 {
			logger.Error("parsing-command", errors.New("drivers json or id not specified"))
			os.Exit(1)
		}

		ctrlPipeR := os.NewFile(3, "/ctrl/pipe")
		buffer := make([]byte, 1)
		logger.Debug("waiting-for-control-pipe")
		if _, err := ctrlPipeR.Read(buffer); err != nil {
			logger.Error("reading-control-pipe", err)
			os.Exit(1)
		}
		logger.Debug("got-back-from-control-pipe")

		var driverSpec spec.DriverSpec
		if err := json.Unmarshal([]byte(os.Args[1]), &driverSpec); err != nil {
			logger.Error("unmarshalling driver spec", err)
			os.Exit(1)
		}

		driver, err := specToDriver(driverSpec)
		if err != nil {
			logger.Error("creating fsdriver", err)
			os.Exit(1)
		}

		volumeID := os.Args[2]
		if err := driver.DestroyVolume(logger, volumeID); err != nil {
			logger.Error("destroying volume", err)
			os.Exit(1)
		}
	})

	reexec.Register("destroy-image", func() {
		cli.ErrWriter = os.Stdout
		logger := lager.NewLogger("destroy-image")
		logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.DEBUG))

		if len(os.Args) != 3 {
			logger.Error("parsing-command", errors.New("drivers json or path not specified"))
			os.Exit(1)
		}

		ctrlPipeR := os.NewFile(3, "/ctrl/pipe")
		buffer := make([]byte, 1)
		logger.Debug("waiting-for-control-pipe")
		if _, err := ctrlPipeR.Read(buffer); err != nil {
			logger.Error("reading-control-pipe", err)
			os.Exit(1)
		}
		logger.Debug("got-back-from-control-pipe")

		var driverSpec spec.DriverSpec
		if err := json.Unmarshal([]byte(os.Args[1]), &driverSpec); err != nil {
			logger.Error("unmarshalling driver spec", err)
			os.Exit(1)
		}

		driver, err := specToDriver(driverSpec)
		if err != nil {
			logger.Error("creating fsdriver", err)
			os.Exit(1)
		}

		imagePath := os.Args[2]
		if err := driver.DestroyImage(logger, imagePath); err != nil {
			logger.Error("destroying image", err)
			os.Exit(1)
		}
	})
}

func (d *Driver) VolumePath(logger lager.Logger, id string) (string, error) {
	return d.driver.VolumePath(logger, id)
}

func (d *Driver) CreateVolume(logger lager.Logger, parentID string, id string) (string, error) {
	return d.driver.CreateVolume(logger, parentID, id)
}

func (d *Driver) DestroyVolume(logger lager.Logger, id string) error {
	if len(d.idMappings.UIDMappings)+len(d.idMappings.GIDMappings) == 0 || os.Getuid() == 0 {
		return d.driver.DestroyVolume(logger, id)
	}

	logger = logger.Session("ns-destroy-volume")
	logger.Debug("starting")
	defer logger.Debug("ending")

	driverJSON, _ := d.driver.Marshal(logger)

	ctrlPipeR, ctrlPipeW, err := os.Pipe()
	if err != nil {
		return errors.Wrap(err, "creating control pipe")
	}

	outputBuffer := bytes.NewBuffer([]byte{})
	cmd := reexec.Command("destroy-volume", string(driverJSON), id)
	cmd.Stderr = lagregator.NewRelogger(logger)
	cmd.Stdout = outputBuffer
	cmd.ExtraFiles = []*os.File{ctrlPipeR}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER,
	}

	logger.Debug("starting-destroy-volume-reexec", lager.Data{"args": cmd.Args})
	if err := d.runner.Start(cmd); err != nil {
		return errors.Wrap(err, "reexecing destroy volume")
	}

	if err := d.idMapper.MapUIDs(logger, cmd.Process.Pid, d.idMappings.UIDMappings); err != nil {
		return errors.Wrap(err, "mapping uids")
	}

	if err := d.idMapper.MapGIDs(logger, cmd.Process.Pid, d.idMappings.GIDMappings); err != nil {
		return errors.Wrap(err, "mapping gids")
	}

	if _, err := ctrlPipeW.Write([]byte{0}); err != nil {
		return errors.Wrap(err, "writing to control pipe")
	}

	if err := d.runner.Wait(cmd); err != nil {
		return errors.Wrapf(err, "waiting for destroy volume rexec: %s", outputBuffer.String())
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
	if len(d.idMappings.UIDMappings)+len(d.idMappings.GIDMappings) == 0 || os.Getuid() == 0 {
		return d.driver.DestroyImage(logger, path)
	}

	logger = logger.Session("ns-destroy-image")
	logger.Debug("starting")
	defer logger.Debug("ending")

	driverJSON, _ := d.driver.Marshal(logger)

	ctrlPipeR, ctrlPipeW, err := os.Pipe()
	if err != nil {
		return errors.Wrap(err, "creating control pipe")
	}

	outputBuffer := bytes.NewBuffer([]byte{})
	cmd := reexec.Command("destroy-image", string(driverJSON), path)
	cmd.Stderr = lagregator.NewRelogger(logger)
	cmd.Stdout = outputBuffer
	cmd.ExtraFiles = []*os.File{ctrlPipeR}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER,
	}

	logger.Debug("starting-destroy-image-reexec", lager.Data{"args": cmd.Args})
	if err := d.runner.Start(cmd); err != nil {
		return errors.Wrap(err, "reexecing destroy image")
	}

	if err := d.idMapper.MapUIDs(logger, cmd.Process.Pid, d.idMappings.UIDMappings); err != nil {
		return errors.Wrap(err, "mapping uids")
	}

	if err := d.idMapper.MapGIDs(logger, cmd.Process.Pid, d.idMappings.GIDMappings); err != nil {
		return errors.Wrap(err, "mapping gids")
	}

	if _, err := ctrlPipeW.Write([]byte{0}); err != nil {
		return errors.Wrap(err, "writing to control pipe")
	}

	if err := d.runner.Wait(cmd); err != nil {
		return errors.Wrapf(err, "waiting for destroy image rexec: %s", outputBuffer.String())
	}

	return nil
}
func (d *Driver) FetchStats(logger lager.Logger, path string) (groot.VolumeStats, error) {
	return d.driver.FetchStats(logger, path)
}

func specToDriver(spec spec.DriverSpec) (internalDriver, error) {
	switch spec.Type {
	case "btrfs":
		return btrfs.NewDriver(
			spec.FsBinaryPath,
			spec.MkfsBinaryPath,
			spec.SuidBinaryPath,
			spec.StorePath), nil
	case "overlay-xfs":
		return overlayxfs.NewDriver(
			spec.StorePath,
			spec.SuidBinaryPath,
			0), nil
	default:
		return nil, errors.Errorf("invalid filesystem spec: %s not recognized", spec.Type)
	}
}

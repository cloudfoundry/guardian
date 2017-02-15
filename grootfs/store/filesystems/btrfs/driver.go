package btrfs // import "code.cloudfoundry.org/grootfs/store/filesystems/btrfs"

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/tscolari/lagregator"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/lager"
)

type Driver struct {
	draxBinPath  string
	btrfsBinPath string
	storePath    string
}

func NewDriver(btrfsBinPath, draxBinPath, storePath string) *Driver {
	return &Driver{
		btrfsBinPath: btrfsBinPath,
		draxBinPath:  draxBinPath,
		storePath:    storePath,
	}
}

func (d *Driver) VolumePath(logger lager.Logger, id string) (string, error) {
	volPath := filepath.Join(d.storePath, store.VolumesDirName, id)
	_, err := os.Stat(volPath)
	if err == nil {
		return volPath, nil
	}

	return "", fmt.Errorf("volume does not exist `%s`: %s", id, err)
}

func (d *Driver) CreateVolume(logger lager.Logger, parentID, id string) (string, error) {
	logger = logger.Session("btrfs-creating-volume", lager.Data{"parentID": parentID, "id": id})
	logger.Info("start")
	defer logger.Info("end")

	var cmd *exec.Cmd
	volPath := filepath.Join(d.storePath, store.VolumesDirName, id)
	if parentID == "" {
		cmd = exec.Command(d.btrfsBinPath, "subvolume", "create", volPath)
	} else {
		parentVolPath := filepath.Join(d.storePath, store.VolumesDirName, parentID)
		cmd = exec.Command(d.btrfsBinPath, "subvolume", "snapshot", parentVolPath, volPath)
	}

	logger.Debug("starting-btrfs", lager.Data{"path": cmd.Path, "args": cmd.Args})
	if contents, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf(
			"creating btrfs volume `%s` (%s): %s",
			volPath, err, string(contents),
		)
	}

	return volPath, nil
}

func (d *Driver) CreateImage(logger lager.Logger, spec image_cloner.ImageDriverSpec) error {
	logger = logger.Session("btrfs-creating-snapshot", lager.Data{"spec": spec})
	logger.Info("start")
	defer logger.Info("end")

	toPath := filepath.Join(spec.ImagePath, "rootfs")
	baseVolumePath := filepath.Join(d.storePath, store.VolumesDirName, spec.BaseVolumeIDs[len(spec.BaseVolumeIDs)-1])
	cmd := exec.Command(d.btrfsBinPath, "subvolume", "snapshot", baseVolumePath, toPath)

	logger.Debug("starting-btrfs", lager.Data{"path": cmd.Path, "args": cmd.Args})
	if contents, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf(
			"creating btrfs snapshot from `%s` to `%s` (%s): %s",
			baseVolumePath, toPath, err, string(contents),
		)
	}

	return d.applyDiskLimit(logger, spec)
}

func (d *Driver) Volumes(logger lager.Logger) ([]string, error) {
	volumes := []string{}

	existingVolumes, err := ioutil.ReadDir(path.Join(d.storePath, store.VolumesDirName))
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %s", err.Error())
	}

	for _, volumeInfo := range existingVolumes {
		volumes = append(volumes, volumeInfo.Name())
	}

	return volumes, nil
}

func (d *Driver) destroyQgroup(logger lager.Logger, path string) error {
	if !d.draxInPath() {
		logger.Info("drax-command-not-found", lager.Data{
			"warning": "could not delete quota group",
		})

		return nil
	}

	if !d.hasSUID() {
		return errors.New("missing the setuid bit on drax")
	}

	cmd := exec.Command(d.draxBinPath, "--btrfs-bin", d.btrfsBinPath, "destroy", "--volume-path", path)
	stdoutBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = stdoutBuffer
	cmd.Stderr = lagregator.NewRelogger(logger)

	logger.Debug("starting-drax", lager.Data{"path": cmd.Path, "args": cmd.Args})
	err := cmd.Run()
	if err != nil {
		logger.Error("drax-failed", err)
		return fmt.Errorf("destroying quota group (%s): %s", err, strings.TrimSpace(stdoutBuffer.String()))
	}

	return nil
}

func (d *Driver) DestroyVolume(logger lager.Logger, id string) error {
	logger = logger.Session("btrfs-destroying-volume", lager.Data{"volumeID": id})
	logger.Info("start")
	defer logger.Info("end")

	return d.destroyBtrfsVolume(logger, filepath.Join(d.storePath, "volumes", id))
}

func (d *Driver) DestroyImage(logger lager.Logger, imagePath string) error {
	logger = logger.Session("btrfs-destroying-image", lager.Data{"imagePath": imagePath})
	logger.Info("start")
	defer logger.Info("end")

	return d.destroyBtrfsVolume(logger, filepath.Join(imagePath, "rootfs"))
}

func (d *Driver) destroyBtrfsVolume(logger lager.Logger, path string) error {
	logger = logger.Session("destroying-subvolume", lager.Data{"path": path})
	logger.Info("start")
	defer logger.Info("end")

	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("image path not found: %s", err)
	}

	if err := d.destroyQgroup(logger, path); err != nil {
		logger.Error("destroying-quota-groups-failed", err)
	}

	cmd := exec.Command(d.btrfsBinPath, "subvolume", "delete", path)
	logger.Debug("starting-btrfs", lager.Data{"path": cmd.Path, "args": cmd.Args})
	if contents, err := cmd.CombinedOutput(); err != nil {
		logger.Error("btrfs-failed", err)
		return fmt.Errorf("destroying volume (%s): %s", err, strings.TrimSpace(string(contents)))
	}
	return nil
}

func (d *Driver) applyDiskLimit(logger lager.Logger, spec image_cloner.ImageDriverSpec) error {
	logger = logger.Session("applying-quotas", lager.Data{"spec": spec})
	logger.Info("start")
	defer logger.Info("end")

	if spec.DiskLimit == 0 {
		logger.Debug("no-need-for-quotas")
		return nil
	}

	if !d.draxInPath() {
		return errors.New("drax was not found in the $PATH")
	}

	if !d.hasSUID() {
		return errors.New("missing the setuid bit on drax")
	}

	args := []string{
		"--btrfs-bin", d.btrfsBinPath,
		"limit",
		"--volume-path", filepath.Join(spec.ImagePath, "rootfs"),
		"--disk-limit-bytes", strconv.FormatInt(spec.DiskLimit, 10),
	}

	if spec.ExclusiveDiskLimit {
		args = append(args, "--exclude-image-from-quota")
	}

	cmd := exec.Command(d.draxBinPath, args...)
	stdoutBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = stdoutBuffer
	cmd.Stderr = lagregator.NewRelogger(logger)

	logger.Debug("starting-drax", lager.Data{"path": cmd.Path, "args": cmd.Args})
	err := cmd.Run()

	if err != nil {
		logger.Error("drax-failed", err)
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(stdoutBuffer.String()))
	}

	return nil
}

func (d *Driver) FetchStats(logger lager.Logger, imagePath string) (groot.VolumeStats, error) {
	logger = logger.Session("btrfs-fetching-stats", lager.Data{"imagePath": imagePath})
	logger.Info("start")
	defer logger.Info("end")

	if !d.draxInPath() {
		return groot.VolumeStats{}, errors.New("drax was not found in the $PATH")
	}

	if !d.hasSUID() {
		return groot.VolumeStats{}, errors.New("missing the setuid bit on drax")
	}

	args := []string{
		"--btrfs-bin", d.btrfsBinPath,
		"stats",
		"--volume-path", filepath.Join(imagePath, "rootfs"),
		"--force-sync",
	}

	cmd := exec.Command(d.draxBinPath, args...)
	stdoutBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = stdoutBuffer
	cmd.Stderr = lagregator.NewRelogger(logger)
	err := cmd.Run()
	if err != nil {
		logger.Error("drax-failed", err)
		return groot.VolumeStats{}, fmt.Errorf("%s: %s", err, strings.TrimSpace(stdoutBuffer.String()))
	}

	usageRegexp := regexp.MustCompile(`.*\s+(\d+)\s+(\d+)$`)
	usage := usageRegexp.FindStringSubmatch(strings.TrimSpace(stdoutBuffer.String()))

	var stats groot.VolumeStats
	if len(usage) != 3 {
		logger.Error("parsing-stats-failed", fmt.Errorf("raw stats: %s", stdoutBuffer.String()))
		return stats, errors.New("could not parse stats")
	}

	fmt.Sscanf(usage[1], "%d", &stats.DiskUsage.TotalBytesUsed)
	fmt.Sscanf(usage[2], "%d", &stats.DiskUsage.ExclusiveBytesUsed)

	return stats, nil
}

func (d *Driver) draxInPath() bool {
	if _, err := exec.LookPath(d.draxBinPath); err != nil {
		return false
	}
	return true
}

func (d *Driver) hasSUID() bool {
	path, err := exec.LookPath(d.draxBinPath)
	if err != nil {
		return false
	}
	// If LookPath succeeds Stat cannot fail
	draxInfo, _ := os.Stat(path)
	if (draxInfo.Mode() & os.ModeSetuid) == 0 {
		return false
	}
	return true
}

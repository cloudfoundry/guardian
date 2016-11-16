package volume_driver // import "code.cloudfoundry.org/grootfs/store/volume_driver"

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
	"code.cloudfoundry.org/lager"
)

type Btrfs struct {
	draxBinPath string
	storePath   string
}

func NewBtrfs(draxBinPath, storePath string) *Btrfs {
	return &Btrfs{
		draxBinPath: draxBinPath,
		storePath:   storePath,
	}
}

func (d *Btrfs) Path(logger lager.Logger, id string) (string, error) {
	volPath := filepath.Join(d.storePath, store.VOLUMES_DIR_NAME, id)
	_, err := os.Stat(volPath)
	if err == nil {
		return volPath, nil
	}

	return "", fmt.Errorf("volume does not exist `%s`: %s", id, err)
}

func (d *Btrfs) Create(logger lager.Logger, parentID, id string) (string, error) {
	logger = logger.Session("btrfs-creating-volume", lager.Data{"parentID": parentID, "id": id})
	logger.Info("start")
	defer logger.Info("end")

	var cmd *exec.Cmd
	volPath := filepath.Join(d.storePath, store.VOLUMES_DIR_NAME, id)
	if parentID == "" {
		cmd = exec.Command("btrfs", "subvolume", "create", volPath)
	} else {
		parentVolPath := filepath.Join(d.storePath, store.VOLUMES_DIR_NAME, parentID)
		cmd = exec.Command("btrfs", "subvolume", "snapshot", parentVolPath, volPath)
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

func (d *Btrfs) Snapshot(logger lager.Logger, fromPath, toPath string) error {
	logger = logger.Session("btrfs-creating-snapshot", lager.Data{"fromPath": fromPath, "toPath": toPath})
	logger.Info("start")
	defer logger.Info("end")

	cmd := exec.Command("btrfs", "subvolume", "snapshot", fromPath, toPath)

	logger.Debug("starting-btrfs", lager.Data{"path": cmd.Path, "args": cmd.Args})
	if contents, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf(
			"creating btrfs snapshot from `%s` to `%s` (%s): %s",
			fromPath, toPath, err, string(contents),
		)
	}

	return nil
}

func (d *Btrfs) Volumes(logger lager.Logger) ([]string, error) {
	volumes := []string{}

	existingVolumes, err := ioutil.ReadDir(path.Join(d.storePath, store.VOLUMES_DIR_NAME))
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %s", err.Error())
	}

	for _, volumeInfo := range existingVolumes {
		volumes = append(volumes, volumeInfo.Name())
	}

	return volumes, nil
}

func (d *Btrfs) destroyQgroup(logger lager.Logger, path string) error {
	if !d.draxInPath() {
		logger.Info("drax-command-not-found", lager.Data{
			"warning": "could not delete quota group",
		})

		return nil
	}

	if !d.hasSUID() {
		return errors.New("missing the setuid bit on drax")
	}

	cmd := exec.Command(d.draxBinPath, "destroy", "--volume-path", path)
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

func (d *Btrfs) DestroyVolume(logger lager.Logger, id string) error {
	logger = logger.Session("btrfs-destroying-volume", lager.Data{"volumeID": id})
	logger.Info("start")
	defer logger.Info("end")

	return d.Destroy(logger, filepath.Join(d.storePath, "volumes", id))
}

func (d *Btrfs) Destroy(logger lager.Logger, path string) error {
	logger = logger.Session("btrfs-destroying", lager.Data{"path": path})
	logger.Info("start")
	defer logger.Info("end")

	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("image path not found: %s", err)
	}

	if err := d.destroyQgroup(logger, path); err != nil {
		logger.Error("destroying-quota-groups-failed", err)
	}

	cmd := exec.Command("btrfs", "subvolume", "delete", path)
	logger.Debug("starting-btrfs", lager.Data{"path": cmd.Path, "args": cmd.Args})
	if contents, err := cmd.CombinedOutput(); err != nil {
		logger.Error("btrfs-failed", err)
		return fmt.Errorf("destroying volume (%s): %s", err, strings.TrimSpace(string(contents)))
	}
	return nil
}

func (d *Btrfs) ApplyDiskLimit(logger lager.Logger, path string, diskLimit int64, excludeImageFromQuota bool) error {
	logger = logger.Session("btrfs-applying-quotas", lager.Data{"path": path})
	logger.Info("start")
	defer logger.Info("end")

	if !d.draxInPath() {
		return errors.New("drax was not found in the $PATH")
	}

	if !d.hasSUID() {
		return errors.New("missing the setuid bit on drax")
	}

	args := []string{
		"limit",
		"--volume-path", path,
		"--disk-limit-bytes", strconv.FormatInt(diskLimit, 10),
	}

	if excludeImageFromQuota {
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

func (d *Btrfs) FetchMetrics(logger lager.Logger, path string) (groot.VolumeMetrics, error) {
	logger = logger.Session("btrfs-fetching-metrics", lager.Data{"path": path})
	logger.Info("start")
	defer logger.Info("end")

	if !d.draxInPath() {
		return groot.VolumeMetrics{}, errors.New("drax was not found in the $PATH")
	}

	if !d.hasSUID() {
		return groot.VolumeMetrics{}, errors.New("missing the setuid bit on drax")
	}

	args := []string{
		"metrics",
		"--volume-path", path,
		"--force-sync",
	}

	cmd := exec.Command(d.draxBinPath, args...)
	stdoutBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = stdoutBuffer
	cmd.Stderr = lagregator.NewRelogger(logger)
	err := cmd.Run()
	if err != nil {
		logger.Error("drax-failed", err)
		return groot.VolumeMetrics{}, fmt.Errorf("%s: %s", err, strings.TrimSpace(stdoutBuffer.String()))
	}

	usageRegexp := regexp.MustCompile(`.*\s+(\d+)\s+(\d+)$`)
	usage := usageRegexp.FindStringSubmatch(strings.TrimSpace(stdoutBuffer.String()))

	var metrics groot.VolumeMetrics
	if len(usage) != 3 {
		logger.Error("parsing-metrics-failed", fmt.Errorf("raw metrics: %s", stdoutBuffer.String()))
		return metrics, errors.New("could not parse metrics")
	}

	fmt.Sscanf(usage[1], "%d", &metrics.DiskUsage.TotalBytesUsed)
	fmt.Sscanf(usage[2], "%d", &metrics.DiskUsage.ExclusiveBytesUsed)

	return metrics, nil
}

func (d *Btrfs) draxInPath() bool {
	if _, err := exec.LookPath(d.draxBinPath); err != nil {
		return false
	}
	return true
}

func (d *Btrfs) hasSUID() bool {
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

package volume_driver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"
)

type Btrfs struct {
	storePath string
}

func NewBtrfs(storePath string) *Btrfs {
	return &Btrfs{
		storePath: storePath,
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
		return fmt.Errorf("bundle path not found: %s", err)
	}

	var cmd *exec.Cmd
	if _, err := exec.LookPath("drax"); err != nil {
		logger.Info("drax-command-not-found", lager.Data{
			"warning": "could not delete quota group",
		})
	} else {
		cmd = exec.Command("drax", "destroy", "--volume-path", path)
		stdoutBuffer := bytes.NewBuffer([]byte{})
		cmd.Stdout = stdoutBuffer
		stderrBuffer := bytes.NewBuffer([]byte{})
		cmd.Stderr = stderrBuffer

		logger.Debug("starting-drax", lager.Data{"path": cmd.Path, "args": cmd.Args})
		err := cmd.Run()
		d.relogStream(logger, stderrBuffer)
		if err != nil {
			logger.Error("drax-failed", err)
			return fmt.Errorf("destroying quota group (%s): %s", err, strings.TrimSpace(stdoutBuffer.String()))
		}
	}

	cmd = exec.Command("btrfs", "subvolume", "delete", path)
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

	args := []string{
		"limit",
		"--volume-path", path,
		"--disk-limit-bytes", strconv.FormatInt(diskLimit, 10),
	}

	if excludeImageFromQuota {
		args = append(args, "--exclude-image-from-quota")
	}

	cmd := exec.Command("drax", args...)
	stdoutBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = stdoutBuffer
	stderrBuffer := bytes.NewBuffer([]byte{})
	cmd.Stderr = stderrBuffer

	logger.Debug("starting-drax", lager.Data{"path": cmd.Path, "args": cmd.Args})
	err := cmd.Run()
	d.relogStream(logger, stderrBuffer)

	if err != nil {
		logger.Error("drax-failed", err)
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(stdoutBuffer.String()))
	}

	return nil
}

func (d *Btrfs) FetchMetrics(logger lager.Logger, path string, forceSync bool) (groot.VolumeMetrics, error) {
	logger = logger.Session("btrfs-fetching-metrics", lager.Data{"path": path})
	logger.Info("start")
	defer logger.Info("end")

	args := []string{
		"metrics",
		"--volume-path", path,
	}
	if forceSync {
		args = append(args, "--force-sync")
	}

	cmd := exec.Command("drax", args...)
	output, err := cmd.Output()
	if err != nil {
		logger.Error("drax-failed", err)
		return groot.VolumeMetrics{}, fmt.Errorf("%s: %s", err, strings.TrimSpace(string(output)))
	}

	usageRegexp := regexp.MustCompile(`.*\s+(\d+)\s+(\d+)$`)
	usage := usageRegexp.FindStringSubmatch(strings.TrimSpace(string(output)))

	var metrics groot.VolumeMetrics
	if len(usage) != 3 {
		logger.Error("parsing-metrics-failed", fmt.Errorf("raw metrics: %s", string(output)))
		return metrics, errors.New("could not parse metrics")
	}

	fmt.Sscanf(usage[1], "%d", &metrics.DiskUsage.TotalBytesUsed)
	fmt.Sscanf(usage[2], "%d", &metrics.DiskUsage.ExclusiveBytesUsed)

	return metrics, nil
}

func (d *Btrfs) relogStream(logger lager.Logger, stream io.Reader) {
	decoder := json.NewDecoder(stream)

	var logFormat lager.LogFormat
	for {
		if err := decoder.Decode(&logFormat); err != nil {
			break
		}

		logger.Debug(logFormat.Message, lager.Data{
			"timestamp": logFormat.Timestamp,
			"source":    logFormat.Source,
			"logLevel":  logFormat.LogLevel,
			"data":      logFormat.Data,
		})
	}
}

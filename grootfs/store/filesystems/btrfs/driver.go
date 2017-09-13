package btrfs // import "code.cloudfoundry.org/grootfs/store/filesystems/btrfs"

import (
	"bytes"
	"encoding/json"
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

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/filesystems"
	"code.cloudfoundry.org/grootfs/store/filesystems/spec"
	"code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

const (
	BtrfsType = 0x9123683E
)

type Driver struct {
	draxBinPath  string
	btrfsBinPath string
	mkfsBinPath  string
	storePath    string
}

func NewDriver(btrfsBinPath, mkfsBinPath, draxBinPath, storePath string) *Driver {
	return &Driver{
		btrfsBinPath: btrfsBinPath,
		mkfsBinPath:  mkfsBinPath,
		draxBinPath:  draxBinPath,
		storePath:    storePath,
	}
}

func (d *Driver) InitFilesystem(logger lager.Logger, filesystemPath, storePath string) error {
	logger = logger.Session("btrfs-init-filesystem")
	logger.Debug("starting")
	defer logger.Debug("ending")

	if err := d.mountFilesystem("remount", filesystemPath, storePath); err != nil {
		if err := d.formatFilesystem(logger, filesystemPath); err != nil {
			return err
		}
		if err := d.mountFilesystem("", filesystemPath, storePath); err != nil {
			return errorspkg.Errorf("Mounting filesystem: %s", err)
		}
	}

	return nil
}

func (d *Driver) ConfigureStore(logger lager.Logger, storePath string, ownerUID, ownerGID int) error {
	return nil
}

func (d *Driver) ValidateFileSystem(logger lager.Logger, path string) error {
	logger = logger.Session("btrfs-validate-filesystem", lager.Data{"path": path})
	logger.Debug("starting")
	defer logger.Debug("ending")

	if err := filesystems.CheckFSPath(path, "btrfs", "user_subvol_rm_allowed"); err != nil {
		logger.Error("validating-filesystem", err)
		return errorspkg.Wrap(err, "btrfs filesystem validation")
	}

	return nil
}

func (d *Driver) VolumePath(logger lager.Logger, id string) (string, error) {
	volPath := filepath.Join(d.storePath, store.VolumesDirName, id)
	_, err := os.Stat(volPath)
	if err == nil {
		return volPath, nil
	}

	return "", errorspkg.Wrapf(err, "volume does not exist `%s`", id)
}

func (d *Driver) CreateVolume(logger lager.Logger, parentID, id string) (string, error) {
	logger = logger.Session("btrfs-creating-volume", lager.Data{"parentID": parentID, "id": id})
	logger.Info("starting")
	defer logger.Info("ending")

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
		return "", errorspkg.Wrapf(err, "creating btrfs volume `%s` %s", volPath, string(contents))
	}

	return volPath, nil
}

func (d *Driver) MoveVolume(logger lager.Logger, from, to string) error {
	logger = logger.Session("btrfs-moving-volume", lager.Data{"from": from, "to": to})
	logger.Debug("starting")
	defer logger.Debug("ending")

	if err := os.Rename(from, to); err != nil {
		if !os.IsExist(err) {
			logger.Error("moving-volume-failed", err)
			return errorspkg.Wrap(err, "moving volume")
		}
	}

	return nil
}

func (d *Driver) HandleOpaqueWhiteouts(logger lager.Logger, id string, opaqueWhiteouts []string) error {
	volumePath, err := d.VolumePath(logger, id)
	if err != nil {
		return err
	}

	for _, opaqueWhiteout := range opaqueWhiteouts {
		parentDir := path.Dir(filepath.Join(volumePath, opaqueWhiteout))
		if err := cleanWhiteoutDir(parentDir); err != nil {
			return err
		}
	}

	return nil
}

func cleanWhiteoutDir(path string) error {
	contents, err := ioutil.ReadDir(path)
	if err != nil {
		return errorspkg.Wrap(err, "reading whiteout directory")
	}

	for _, content := range contents {
		if err := os.RemoveAll(filepath.Join(path, content.Name())); err != nil {
			return errorspkg.Wrap(err, "cleaning up whiteout directory")
		}
	}

	return nil
}

func (d *Driver) WriteVolumeMeta(_ lager.Logger, _ string, _ base_image_puller.VolumeMeta) error {
	return nil
}

func (d *Driver) CreateImage(logger lager.Logger, spec image_cloner.ImageDriverSpec) (groot.MountInfo, error) {
	logger = logger.Session("btrfs-creating-snapshot", lager.Data{"spec": spec})
	logger.Info("starting")
	defer logger.Info("ending")

	toPath := filepath.Join(spec.ImagePath, "rootfs")
	baseVolumePath := filepath.Join(d.storePath, store.VolumesDirName, spec.BaseVolumeIDs[len(spec.BaseVolumeIDs)-1])
	var mountInfo groot.MountInfo

	if !spec.Mount {
		if err := os.Mkdir(toPath, 0755); err != nil {
			logger.Error("creating-rootfs-folder-failed", err, lager.Data{"rootfs": toPath})
			return groot.MountInfo{}, errorspkg.Wrap(err, "creating rootfs folder")
		}

		mountInfo.Destination = toPath
		mountInfo.Type = ""
		mountInfo.Source = filepath.Join(spec.ImagePath, "snapshot")
		mountInfo.Options = []string{"bind"}

		toPath = mountInfo.Source
	}

	cmd := exec.Command(d.btrfsBinPath, "subvolume", "snapshot", baseVolumePath, toPath)
	logger.Debug("starting-btrfs", lager.Data{"path": cmd.Path, "args": cmd.Args})
	if contents, err := cmd.CombinedOutput(); err != nil {
		return groot.MountInfo{}, errorspkg.Errorf(
			"creating btrfs snapshot from `%s` to `%s` (%s): %s",
			baseVolumePath, toPath, err, string(contents),
		)
	}

	return mountInfo, d.applyDiskLimit(logger, spec)
}

func (d *Driver) Volumes(logger lager.Logger) ([]string, error) {
	logger = logger.Session("btrfs-listing-volumes")
	logger.Debug("starting")
	defer logger.Debug("ending")

	volumes := []string{}
	existingVolumes, err := ioutil.ReadDir(path.Join(d.storePath, store.VolumesDirName))
	if err != nil {
		return nil, errorspkg.Wrap(err, "failed to list volumes")
	}

	for _, volumeInfo := range existingVolumes {
		volumes = append(volumes, volumeInfo.Name())
	}

	return volumes, nil
}

func (d *Driver) DestroyVolume(logger lager.Logger, id string) error {
	logger = logger.Session("btrfs-destroying-volume", lager.Data{"volumeID": id})
	logger.Info("starting")
	defer logger.Info("ending")

	return d.destroyBtrfsVolume(logger, filepath.Join(d.storePath, "volumes", id))
}

func (d *Driver) DestroyImage(logger lager.Logger, imagePath string) error {
	logger = logger.Session("btrfs-destroying-image", lager.Data{"imagePath": imagePath})
	logger.Info("starting")
	defer logger.Info("ending")

	snapshotMountPath := filepath.Join(imagePath, "rootfs")
	if _, err := os.Stat(filepath.Join(imagePath, "snapshot")); err == nil {
		if err := os.Remove(snapshotMountPath); err != nil {
			logger.Error("removing-rootfs-folder-failed", err)
			return errorspkg.Wrap(err, "remove rootfs folder")
		}
		snapshotMountPath = filepath.Join(imagePath, "snapshot")
	}

	err := d.destroyBtrfsVolume(logger, snapshotMountPath)
	if err != nil && strings.Contains(err.Error(), "Directory not empty") {
		subvolumes, err := d.listSubvolumes(logger, imagePath)
		if err != nil {
			logger.Error("listing-subvolumes-failed", err)
			return errorspkg.Wrap(err, "list subvolumes")
		}

		for _, subvolume := range subvolumes {
			if err := d.destroyBtrfsVolume(logger, subvolume); err != nil {
				return err
			}
		}
		return nil
	}

	return err
}

func (d *Driver) FetchStats(logger lager.Logger, imagePath string) (groot.VolumeStats, error) {
	logger = logger.Session("btrfs-fetching-stats", lager.Data{"imagePath": imagePath})
	logger.Debug("starting")
	defer logger.Debug("ending")

	if !d.draxInPath() {
		return groot.VolumeStats{}, errorspkg.New("drax was not found in the $PATH")
	}

	if !d.hasSUID() {
		return groot.VolumeStats{}, errorspkg.New("missing the setuid bit on drax")
	}

	args := []string{
		"--btrfs-bin", d.btrfsBinPath,
		"stats",
		"--volume-path", filepath.Join(imagePath, "rootfs"),
		"--force-sync",
	}

	stdoutBuffer, err := d.runDrax(logger, args...)
	if err != nil {
		return groot.VolumeStats{}, err
	}

	usageRegexp := regexp.MustCompile(`.*\s+(\d+)\s+(\d+)$`)
	usage := usageRegexp.FindStringSubmatch(strings.TrimSpace(stdoutBuffer.String()))

	var stats groot.VolumeStats
	if len(usage) != 3 {
		logger.Error("parsing-stats-failed", errorspkg.Errorf("raw stats: %s", stdoutBuffer.String()))
		return stats, errorspkg.New("could not parse stats")
	}

	fmt.Sscanf(usage[1], "%d", &stats.DiskUsage.TotalBytesUsed)
	fmt.Sscanf(usage[2], "%d", &stats.DiskUsage.ExclusiveBytesUsed)

	return stats, nil
}

func (d *Driver) Marshal(logger lager.Logger) ([]byte, error) {
	driverSpec := spec.DriverSpec{
		Type:           "btrfs",
		StorePath:      d.storePath,
		FsBinaryPath:   d.btrfsBinPath,
		MkfsBinaryPath: d.mkfsBinPath,
		SuidBinaryPath: d.draxBinPath,
	}

	return json.Marshal(driverSpec)
}

func (d *Driver) formatFilesystem(logger lager.Logger, filesystemPath string) error {
	logger = logger.Session("formatting-filesystem")
	logger.Debug("starting")
	defer logger.Debug("ending")

	stdout := bytes.NewBuffer([]byte{})
	stderr := bytes.NewBuffer([]byte{})
	cmd := exec.Command(d.mkfsBinPath, "-f", filesystemPath)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		logger.Error("formatting-filesystem-failed", err, lager.Data{"cmd": cmd.Args, "stdout": stdout.String(), "stderr": stderr.String()})
		return errorspkg.Errorf("Formatting BTRFS filesystem: %s", err.Error())
	}

	return nil
}

func (d *Driver) mountFilesystem(option, source, destination string) error {
	allOpts := strings.Trim(fmt.Sprintf("%s,user_subvol_rm_allowed,rw", option), ",")

	cmd := exec.Command("mount", "-o", allOpts, "-t", "btrfs", source, destination)
	if output, err := cmd.CombinedOutput(); err != nil {
		return errorspkg.Errorf("%s: %s", err, string(output))
	}

	return nil
}

func (d *Driver) destroyBtrfsVolume(logger lager.Logger, path string) error {
	logger = logger.Session("destroying-subvolume", lager.Data{"path": path})
	logger.Info("starting")
	defer logger.Info("ending")

	if _, err := os.Stat(path); err != nil {
		return errorspkg.Wrap(err, "image path not found")
	}

	if err := d.destroyQgroup(logger, path); err != nil {
		logger.Error("destroying-quota-groups-failed", err, lager.Data{
			"warning": "could not delete quota group"})
	}

	cmd := exec.Command(d.btrfsBinPath, "subvolume", "delete", path)
	logger.Debug("starting-btrfs", lager.Data{"path": cmd.Path, "args": cmd.Args})
	if contents, err := cmd.CombinedOutput(); err != nil {
		logger.Error("btrfs-failed", err)
		return errorspkg.Wrapf(err, "destroying volume %s", strings.TrimSpace(string(contents)))
	}
	return nil
}

func (d *Driver) destroyQgroup(logger lager.Logger, path string) error {
	_, err := d.runDrax(logger, "--btrfs-bin", d.btrfsBinPath, "destroy", "--volume-path", path)

	return err
}

func (d *Driver) listSubvolumes(logger lager.Logger, path string) ([]string, error) {
	logger = logger.Session("listing-subvolumes", lager.Data{"path": path})
	logger.Debug("starting")
	defer logger.Debug("ending")

	args := []string{
		"--btrfs-bin", d.btrfsBinPath,
		"list",
		path,
	}

	stdoutBuffer, err := d.runDrax(logger, args...)
	if err != nil {
		return nil, err
	}

	contents, err := ioutil.ReadAll(stdoutBuffer)
	if err != nil {
		return nil, errorspkg.Wrapf(err, "read drax read output")
	}

	return strings.Split(string(contents), "\n"), nil
}

func (d *Driver) applyDiskLimit(logger lager.Logger, spec image_cloner.ImageDriverSpec) error {
	logger = logger.Session("applying-quotas", lager.Data{"spec": spec})
	logger.Info("starting")
	defer logger.Info("ending")

	if spec.DiskLimit == 0 {
		logger.Debug("no-need-for-quotas")
		return nil
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

	if _, err := d.runDrax(logger, args...); err != nil {
		return err
	}

	return nil
}

func (d *Driver) runDrax(logger lager.Logger, args ...string) (*bytes.Buffer, error) {
	logger = logger.Session("run-drax", lager.Data{"args": args})
	logger.Debug("starting")
	defer logger.Debug("ending")

	if !d.draxInPath() {
		return nil, errorspkg.New("drax was not found in the $PATH")
	}

	if !d.hasSUID() {
		return nil, errorspkg.New("missing the setuid bit on drax")
	}

	cmd := exec.Command(d.draxBinPath, args...)
	stdoutBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = stdoutBuffer
	cmd.Stderr = lagregator.NewRelogger(logger)

	logger.Debug("starting-drax", lager.Data{"path": cmd.Path, "args": cmd.Args})
	err := cmd.Run()

	if err != nil {
		logger.Error("drax-failed", err)
		return nil, errorspkg.Wrapf(err, " %s", strings.TrimSpace(stdoutBuffer.String()))
	}

	return stdoutBuffer, nil
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

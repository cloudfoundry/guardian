package overlayxfs

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/pkg/errors"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	quotapkg "code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs/quota"
	"code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/lager"
)

const (
	UpperDir  = "diff"
	WorkDir   = "workdir"
	RootfsDir = "rootfs"

	imageInfoName = "image_info"
)

func NewDriver(xfsProgsPath, storePath string) *Driver {
	return &Driver{
		xfsProgsPath: xfsProgsPath,
		storePath:    storePath,
	}
}

type Driver struct {
	xfsProgsPath string
	storePath    string
}

func (d *Driver) VolumePath(logger lager.Logger, id string) (string, error) {
	volPath := filepath.Join(d.storePath, store.VolumesDirName, id)
	_, err := os.Stat(volPath)
	if err == nil {
		return volPath, nil
	}

	return "", fmt.Errorf("volume does not exist `%s`: %s", id, err)
}

func (d *Driver) CreateVolume(logger lager.Logger, parentID string, id string) (string, error) {
	logger = logger.Session("overlayxfs-creating-volume", lager.Data{"parentID": parentID, "id": id})
	logger.Info("start")
	defer logger.Info("end")

	volumePath := filepath.Join(d.storePath, store.VolumesDirName, id)
	if err := os.Mkdir(volumePath, 0700); err != nil {
		logger.Error("creating-volume-dir-failed", err)
		return "", errors.Wrap(err, "creating volume")
	}
	return volumePath, nil
}

func (d *Driver) DestroyVolume(logger lager.Logger, id string) error {
	volumePath := filepath.Join(d.storePath, "volumes", id)
	if err := os.RemoveAll(volumePath); err != nil {
		logger.Error(fmt.Sprintf("failed to destroy volume %s", volumePath), err)
		return errors.Wrap(err, fmt.Sprintf("destroying volume (%s)", id))
	}
	return nil
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

func (d *Driver) CreateImage(logger lager.Logger, spec image_cloner.ImageDriverSpec) error {
	logger = logger.Session("overlayxfs-creating-image", lager.Data{"spec": spec})
	logger.Info("start")
	defer logger.Info("end")

	if _, err := os.Stat(spec.ImagePath); os.IsNotExist(err) {
		logger.Error("image-path-not-found", err)
		return errors.Wrap(err, "image path does not exist")
	}

	baseVolumePaths := []string{}
	var baseVolumeSize int64
	for i := len(spec.BaseVolumeIDs) - 1; i >= 0; i-- {
		volumePath := filepath.Join(d.storePath, store.VolumesDirName, spec.BaseVolumeIDs[i])

		if _, err := os.Stat(volumePath); os.IsNotExist(err) {
			logger.Error("base-volume-path-not-found", err)
			return errors.Wrap(err, "base volume path does not exist")
		}

		volumeSize, err := d.duUsage(logger, volumePath)
		if err != nil {
			logger.Error("calculating-base-volume-size-failed", err)
			return errors.Wrapf(err, "calculating base volume size %s", volumePath)
		}

		baseVolumeSize += volumeSize
		baseVolumePaths = append(baseVolumePaths, volumePath)
	}

	upperDir := filepath.Join(spec.ImagePath, UpperDir)
	workDir := filepath.Join(spec.ImagePath, WorkDir)
	rootfsDir := filepath.Join(spec.ImagePath, RootfsDir)

	if err := d.applyDiskLimit(logger, spec, baseVolumeSize); err != nil {
		return errors.Wrap(err, "applying disk limits")
	}

	if err := os.Mkdir(upperDir, 0755); err != nil {
		logger.Error("creating-upperdir-folder-failed", err)
		return errors.Wrap(err, "creating upperdir folder")
	}

	if err := os.Mkdir(workDir, 0755); err != nil {
		logger.Error("creating-workdir-folder-failed", err)
		return errors.Wrap(err, "creating workdir folder")
	}

	if err := os.Mkdir(rootfsDir, 0755); err != nil {
		logger.Error("creating-rootfs-folder-failed", err)
		return errors.Wrap(err, "creating rootfs folder")
	}

	mountData := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", strings.Join(baseVolumePaths, ":"), upperDir, workDir)
	if err := syscall.Mount("overlay", rootfsDir, "overlay", 0, mountData); err != nil {
		logger.Error("mounting-overlay-to-rootfs-failed", err, lager.Data{"mountData": mountData, "rootfsDir": rootfsDir})
		return errors.Wrap(err, "mounting overlay")
	}

	// Allows permissions to work for different users inside the fs
	file, err := os.Open(rootfsDir)
	defer file.Close()
	if err != nil {
		return errors.Wrap(err, "reading rootfsDir")
	}
	file.Readdir(-1)
	// Until here

	imageInfoFileName := filepath.Join(spec.ImagePath, imageInfoName)
	if err := ioutil.WriteFile(imageInfoFileName, []byte(strconv.FormatInt(baseVolumeSize, 10)), 0600); err != nil {
		return errors.Wrapf(err, "writing image info %s", imageInfoFileName)
	}

	return nil
}

func (d *Driver) DestroyImage(logger lager.Logger, imagePath string) error {
	logger = logger.Session("overlayxfs-destroying-image", lager.Data{"imagePath": imagePath})
	logger.Info("start")
	defer logger.Info("end")

	if err := syscall.Unmount(filepath.Join(imagePath, RootfsDir), 0); err != nil {
		logger.Error("unmounting-rootfs-folder-failed", err)
		return errors.Wrap(err, "unmounting rootfs folder")
	}
	if err := os.Remove(filepath.Join(imagePath, RootfsDir)); err != nil {
		logger.Error("removing-rootfs-folder-failed", err)
		return errors.Wrap(err, "deleting rootfs folder")
	}
	if err := os.RemoveAll(filepath.Join(imagePath, WorkDir)); err != nil {
		logger.Error("removing-workdir-folder-failed", err)
		return errors.Wrap(err, "deleting workdir folder")
	}
	if err := os.RemoveAll(filepath.Join(imagePath, UpperDir)); err != nil {
		logger.Error("removing-upperdir-folder-failed", err)
		return errors.Wrap(err, "deleting upperdir folder")
	}

	return nil
}

func (d *Driver) applyDiskLimit(logger lager.Logger, spec image_cloner.ImageDriverSpec, volumeSize int64) error {
	logger = logger.Session("applying-quotas", lager.Data{"spec": spec})
	logger.Debug("start")
	defer logger.Debug("end")

	if spec.DiskLimit == 0 {
		logger.Info("no-need-for-quotas")
		return nil
	}

	diskLimit := spec.DiskLimit
	if spec.ExclusiveDiskLimit {
		logger.Info("applying-exclusive-quotas")
	} else {
		logger.Info("applying-inclusive-quotas")
		diskLimit -= volumeSize
		if diskLimit < 0 {
			err := errors.New("disk limit is smaller than volume size")
			logger.Error("applying-inclusive-quota-failed", err, lager.Data{"imagePath": spec.ImagePath})
			return err
		}
	}

	imagesPath := filepath.Join(d.storePath, store.ImageDirName)
	quotaControl, err := quotapkg.NewControl(imagesPath)
	if err != nil {
		logger.Error("creating-quota-control-failed", err, lager.Data{"imagesPath": imagesPath})
		return errors.Wrapf(err, "creating xfs quota control %s", imagesPath)
	}

	quota := quotapkg.Quota{
		Size: uint64(diskLimit),
	}

	if err := quotaControl.SetQuota(spec.ImagePath, quota); err != nil {
		logger.Error("setting-quota-failed", err)
		return errors.Wrapf(err, "setting quota to %s", spec.ImagePath)
	}

	return nil
}

func (d *Driver) FetchStats(logger lager.Logger, imagePath string) (groot.VolumeStats, error) {
	logger = logger.Session("overlayxfs-fetching-stats", lager.Data{"imagePath": imagePath})
	logger.Info("start")
	defer logger.Info("end")

	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		logger.Error("image-path-not-found", err)
		return groot.VolumeStats{}, errors.Wrapf(err, "image path (%s) doesn't exist", imagePath)
	}

	projectID, err := quotapkg.GetProjectID(imagePath)
	if err != nil {
		logger.Error("fetching-project-id-failed", err)
		return groot.VolumeStats{}, errors.Wrapf(err, "fetching project id for %s", imagePath)
	}

	if projectID == 0 {
		logger.Error("image-path-does-not-have-quota-enabled", err)
		return groot.VolumeStats{}, fmt.Errorf("the image doesn't have a quota applied: %s", imagePath)
	}

	exclusiveSize, err := d.listQuotaUsage(logger, projectID)
	if err != nil {
		logger.Error("list-quota-usage-failed", err, lager.Data{"projectID": projectID})
		return groot.VolumeStats{}, errors.Wrapf(err, "listing quota usage %s", imagePath)
	}

	volumeSize, err := d.readImageInfo(logger, imagePath)
	if err != nil {
		logger.Error("reading-image-info-failed", err)
		return groot.VolumeStats{}, errors.Wrapf(err, "reading image info %s", imagePath)
	}

	logger.Debug("usage", lager.Data{"volumeSize": volumeSize, "exclusiveSize": exclusiveSize})

	return groot.VolumeStats{
		DiskUsage: groot.DiskUsage{
			ExclusiveBytesUsed: exclusiveSize,
			TotalBytesUsed:     volumeSize + exclusiveSize,
		},
	}, nil
}

func (d *Driver) listQuotaUsage(logger lager.Logger, projectID uint32) (int64, error) {
	logger = logger.Session("listing-quota-usage", lager.Data{"projectID": projectID})
	logger.Debug("start")
	defer logger.Debug("end")

	quotaCmd := exec.Command(filepath.Join(d.xfsProgsPath, "xfs_quota"), "-x", "-c", fmt.Sprintf("quota -N -p %d", projectID), d.storePath)
	stdoutBuffer := bytes.NewBuffer([]byte{})
	stderrBuffer := bytes.NewBuffer([]byte{})
	quotaCmd.Stdout = stdoutBuffer
	quotaCmd.Stderr = stdoutBuffer
	if err := quotaCmd.Run(); err != nil {
		return 0, errors.Wrapf(err, "failed to fetch xfs quota: %s", stderrBuffer.String())
	}

	output := stdoutBuffer.String()
	parsedOutput := strings.Fields(output)
	if len(parsedOutput) != 7 {
		return 0, fmt.Errorf("quota usage output not as expected: %s", output)
	}

	usedBlocks, err := strconv.ParseInt(parsedOutput[1], 10, 64)
	if err != nil {
		return 0, err
	}

	// xfs_quota output returns 1K-block values, so we need to multiply it for 1024
	return usedBlocks * 1024, nil
}

func (d *Driver) duUsage(logger lager.Logger, path string) (int64, error) {
	logger = logger.Session("du-metrics", lager.Data{"path": path})
	logger.Debug("start")
	defer logger.Debug("end")

	cmd := exec.Command("du", "-bs", path)
	stdoutBuffer := bytes.NewBuffer([]byte{})
	stderrBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = stdoutBuffer
	cmd.Stderr = stdoutBuffer
	if err := cmd.Run(); err != nil {
		logger.Error("du-command-failed", err, lager.Data{"stdout": stdoutBuffer.String(), "stderr": stderrBuffer.String()})
		return 0, errors.Wrapf(err, "du failed: %s", stderrBuffer.String())
	}

	usageString := strings.Split(stdoutBuffer.String(), "\t")[0]
	return strconv.ParseInt(usageString, 10, 64)
}

func (d *Driver) readImageInfo(logger lager.Logger, imagePath string) (int64, error) {
	contents, err := ioutil.ReadFile(filepath.Join(imagePath, imageInfoName))
	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(string(contents), 10, 64)
}

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

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/filesystems"
	quotapkg "code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs/quota"
	"code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

const (
	BaseFileSystemName = "xfs"
	UpperDir           = "diff"
	WorkDir            = "workdir"
	RootfsDir          = "rootfs"

	imageInfoName = "image_info"
)

func NewDriver(storePath string) (*Driver, error) {
	if err := filesystems.CheckFSPath(storePath, filesystems.XfsType, BaseFileSystemName); err != nil {
		return nil, err
	}
	return &Driver{
		storePath: storePath,
	}, nil
}

type Driver struct {
	storePath string
}

func (d *Driver) VolumePath(logger lager.Logger, id string) (string, error) {
	volPath := filepath.Join(d.storePath, store.VolumesDirName, id)
	_, err := os.Stat(volPath)
	if err == nil {
		return volPath, nil
	}

	return "", errorspkg.Wrapf(err, "volume does not exist `%s`", id)
}

func (d *Driver) CreateVolume(logger lager.Logger, parentID string, id string) (string, error) {
	logger = logger.Session("overlayxfs-creating-volume", lager.Data{"parentID": parentID, "id": id})
	logger.Info("starting")
	defer logger.Info("ending")

	volumePath := filepath.Join(d.storePath, store.VolumesDirName, id)
	if err := os.Mkdir(volumePath, 0755); err != nil {
		logger.Error("creating-volume-dir-failed", err)
		return "", errorspkg.Wrap(err, "creating volume")
	}

	if err := os.Chmod(volumePath, 755); err != nil {
		logger.Error("changing-volume-permissions-failed", err)
		return "", errorspkg.Wrap(err, "changing volume permissions")
	}
	return volumePath, nil
}

func (d *Driver) DestroyVolume(logger lager.Logger, id string) error {
	volumePath := filepath.Join(d.storePath, "volumes", id)
	if err := os.RemoveAll(volumePath); err != nil {
		logger.Error(fmt.Sprintf("failed to destroy volume %s", volumePath), err)
		return errorspkg.Wrapf(err, "destroying volume (%s)", id)
	}
	return nil
}

func (d *Driver) Volumes(logger lager.Logger) ([]string, error) {
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

func (d *Driver) CreateImage(logger lager.Logger, spec image_cloner.ImageDriverSpec) error {
	logger = logger.Session("overlayxfs-creating-image", lager.Data{"spec": spec})
	logger.Info("starting")
	defer logger.Info("ending")

	if _, err := os.Stat(spec.ImagePath); os.IsNotExist(err) {
		logger.Error("image-path-not-found", err)
		return errorspkg.Wrap(err, "image path does not exist")
	}

	baseVolumePaths := []string{}
	var baseVolumeSize int64
	for i := len(spec.BaseVolumeIDs) - 1; i >= 0; i-- {
		volumePath := filepath.Join(d.storePath, store.VolumesDirName, spec.BaseVolumeIDs[i])

		if _, err := os.Stat(volumePath); os.IsNotExist(err) {
			logger.Error("base-volume-path-not-found", err)
			return errorspkg.Wrap(err, "base volume path does not exist")
		}

		volumeSize, err := d.duUsage(logger, volumePath)
		if err != nil {
			logger.Error("calculating-base-volume-size-failed", err)
			return errorspkg.Wrapf(err, "calculating base volume size %s", volumePath)
		}

		baseVolumeSize += volumeSize
		baseVolumePaths = append(baseVolumePaths, volumePath)
	}

	upperDir := filepath.Join(spec.ImagePath, UpperDir)
	workDir := filepath.Join(spec.ImagePath, WorkDir)
	rootfsDir := filepath.Join(spec.ImagePath, RootfsDir)

	if err := d.applyDiskLimit(logger, spec, baseVolumeSize); err != nil {
		return errorspkg.Wrap(err, "applying disk limits")
	}

	if err := os.Mkdir(upperDir, 0755); err != nil {
		logger.Error("creating-upperdir-folder-failed", err)
		return errorspkg.Wrap(err, "creating upperdir folder")
	}

	if err := os.Mkdir(workDir, 0755); err != nil {
		logger.Error("creating-workdir-folder-failed", err)
		return errorspkg.Wrap(err, "creating workdir folder")
	}

	if err := os.Mkdir(rootfsDir, 0755); err != nil {
		logger.Error("creating-rootfs-folder-failed", err)
		return errorspkg.Wrap(err, "creating rootfs folder")
	}

	mountData := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", strings.Join(baseVolumePaths, ":"), upperDir, workDir)
	if err := syscall.Mount("overlay", rootfsDir, "overlay", 0, mountData); err != nil {
		logger.Error("mounting-overlay-to-rootfs-failed", err, lager.Data{"mountData": mountData, "rootfsDir": rootfsDir})
		return errorspkg.Wrap(err, "mounting overlay")
	}

	imageInfoFileName := filepath.Join(spec.ImagePath, imageInfoName)
	if err := ioutil.WriteFile(imageInfoFileName, []byte(strconv.FormatInt(baseVolumeSize, 10)), 0600); err != nil {
		return errorspkg.Wrapf(err, "writing image info %s", imageInfoFileName)
	}

	return nil
}

func (d *Driver) DestroyImage(logger lager.Logger, imagePath string) error {
	logger = logger.Session("overlayxfs-destroying-image", lager.Data{"imagePath": imagePath})
	logger.Info("starting")
	defer logger.Info("ending")

	if err := syscall.Unmount(filepath.Join(imagePath, RootfsDir), 0); err != nil {
		logger.Error("unmounting-rootfs-folder-failed", err)
		return errorspkg.Wrap(err, "unmounting rootfs folder")
	}
	if err := os.Remove(filepath.Join(imagePath, RootfsDir)); err != nil {
		logger.Error("removing-rootfs-folder-failed", err)
		return errorspkg.Wrap(err, "deleting rootfs folder")
	}
	if err := os.RemoveAll(filepath.Join(imagePath, WorkDir)); err != nil {
		logger.Error("removing-workdir-folder-failed", err)
		return errorspkg.Wrap(err, "deleting workdir folder")
	}
	if err := os.RemoveAll(filepath.Join(imagePath, UpperDir)); err != nil {
		logger.Error("removing-upperdir-folder-failed", err)
		return errorspkg.Wrap(err, "deleting upperdir folder")
	}

	return nil
}

func (d *Driver) applyDiskLimit(logger lager.Logger, spec image_cloner.ImageDriverSpec, volumeSize int64) error {
	logger = logger.Session("applying-quotas", lager.Data{"spec": spec})
	logger.Debug("starting")
	defer logger.Debug("ending")

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
			err := errorspkg.New("disk limit is smaller than volume size")
			logger.Error("applying-inclusive-quota-failed", err, lager.Data{"imagePath": spec.ImagePath})
			return err
		}
	}

	imagesPath := filepath.Join(d.storePath, store.ImageDirName)
	quotaControl, err := quotapkg.NewControl(imagesPath)
	if err != nil {
		logger.Error("creating-quota-control-failed", err, lager.Data{"imagesPath": imagesPath})
		return errorspkg.Wrapf(err, "creating xfs quota control %s", imagesPath)
	}

	quota := quotapkg.Quota{
		Size: uint64(diskLimit),
	}

	if err := quotaControl.SetQuota(spec.ImagePath, quota); err != nil {
		logger.Error("setting-quota-failed", err)
		return errorspkg.Wrapf(err, "setting quota to %s", spec.ImagePath)
	}

	return nil
}

func (d *Driver) FetchStats(logger lager.Logger, imagePath string) (groot.VolumeStats, error) {
	logger = logger.Session("overlayxfs-fetching-stats", lager.Data{"imagePath": imagePath})
	logger.Info("starting")
	defer logger.Info("ending")

	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		logger.Error("image-path-not-found", err)
		return groot.VolumeStats{}, errorspkg.Wrapf(err, "image path (%s) doesn't exist", imagePath)
	}

	projectID, err := quotapkg.GetProjectID(imagePath)
	if err != nil {
		logger.Error("fetching-project-id-failed", err)
		return groot.VolumeStats{}, errorspkg.Wrapf(err, "fetching project id for %s", imagePath)
	}

	var exclusiveSize int64
	if projectID != 0 {
		exclusiveSize, err = d.listQuotaUsage(logger, imagePath)
		if err != nil {
			logger.Error("list-quota-usage-failed", err, lager.Data{"projectID": projectID})
			return groot.VolumeStats{}, errorspkg.Wrapf(err, "listing quota usage %s", imagePath)
		}
	}

	volumeSize, err := d.readImageInfo(logger, imagePath)
	if err != nil {
		logger.Error("reading-image-info-failed", err)
		return groot.VolumeStats{}, errorspkg.Wrapf(err, "reading image info %s", imagePath)
	}

	logger.Debug("usage", lager.Data{"volumeSize": volumeSize, "exclusiveSize": exclusiveSize})

	return groot.VolumeStats{
		DiskUsage: groot.DiskUsage{
			ExclusiveBytesUsed: exclusiveSize,
			TotalBytesUsed:     volumeSize + exclusiveSize,
		},
	}, nil
}

func (d *Driver) listQuotaUsage(logger lager.Logger, imagePath string) (int64, error) {
	logger = logger.Session("listing-quota-usage", lager.Data{"imagePath": imagePath})
	logger.Debug("starting")
	defer logger.Debug("ending")

	imagesPath := filepath.Join(d.storePath, store.ImageDirName)
	quotaControl, err := quotapkg.NewControl(imagesPath)
	if err != nil {
		logger.Error("creating-quota-control-failed", err)
		return 0, errorspkg.Wrapf(err, "creating quota control")
	}

	var quota quotapkg.Quota
	if err := quotaControl.GetQuota(imagePath, &quota); err != nil {
		logger.Error("getting-quota-failed", err)
		return 0, errorspkg.Wrapf(err, "getting quota %s", imagePath)
	}

	return int64(quota.BCount), nil
}

func (d *Driver) duUsage(logger lager.Logger, path string) (int64, error) {
	logger = logger.Session("du-metrics", lager.Data{"path": path})
	logger.Debug("starting")
	defer logger.Debug("ending")

	cmd := exec.Command("du", "-bs", path)
	stdoutBuffer := bytes.NewBuffer([]byte{})
	stderrBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = stdoutBuffer
	cmd.Stderr = stdoutBuffer
	if err := cmd.Run(); err != nil {
		logger.Error("du-command-failed", err, lager.Data{"stdout": stdoutBuffer.String(), "stderr": stderrBuffer.String()})
		return 0, errorspkg.Wrapf(err, "du failed: %s", stderrBuffer.String())
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

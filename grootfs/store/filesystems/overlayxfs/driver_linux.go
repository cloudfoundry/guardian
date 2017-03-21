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
	shortid "github.com/ventu-io/go-shortid"
)

func (d *Driver) ConfigureStore(logger lager.Logger, path string, ownerUID, ownerGID int) error {
	logger = logger.Session("overlayxfs-configure-store", lager.Data{"path": path})
	logger.Debug("starting")
	defer logger.Debug("ending")

	if err := d.createWhiteoutDevice(logger, path, ownerUID, ownerGID); err != nil {
		logger.Error("creating-whiteout-device-failed", err)
		return errorspkg.Wrap(err, "Creating whiteout device")
	}

	if err := d.validateWhiteoutDevice(path); err != nil {
		logger.Error("whiteout-device-validation-failed", err)
		return errorspkg.Wrap(err, "Invalid whiteout device")
	}

	if err := d.createLinksDirectory(logger, path, ownerUID, ownerGID); err != nil {
		logger.Error("creating-links-directory-failed", err)
		return errorspkg.Wrap(err, "Create links directory")
	}

	return nil
}

func (d *Driver) ValidateFileSystem(logger lager.Logger, path string) error {
	logger = logger.Session("overlayxfs-validate-filesystem", lager.Data{"path": path})
	logger.Debug("starting")
	defer logger.Debug("ending")

	if err := filesystems.CheckFSPath(path, XfsType); err != nil {
		logger.Error("validating-filesystem", err)
		return errorspkg.Wrap(err, "overlay-xfs filesystem validation")
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

func (d *Driver) CreateVolume(logger lager.Logger, parentID string, id string) (string, error) {
	logger = logger.Session("overlayxfs-creating-volume", lager.Data{"parentID": parentID, "id": id})
	logger.Info("starting")
	defer logger.Info("ending")

	volumePath := filepath.Join(d.storePath, store.VolumesDirName, id)
	if err := os.Mkdir(volumePath, 0755); err != nil {
		logger.Error("creating-volume-dir-failed", err)
		return "", errorspkg.Wrap(err, "creating volume")
	}

	shortId, err := d.generateID()
	if err != nil {
		logger.Error("generating-short-id-failed", err)
		return "", errorspkg.Wrap(err, "generating short id")
	}
	if err := os.Symlink(volumePath, filepath.Join(d.storePath, LinksDirName, shortId)); err != nil {
		logger.Error("creating-volume-symlink-failed", err)
		return "", errorspkg.Wrap(err, "creating volume symlink")
	}
	if err := ioutil.WriteFile(filepath.Join(d.storePath, LinksDirName, id), []byte(shortId), 0644); err != nil {
		logger.Error("creating-link-file-failed", err)
		return "", errorspkg.Wrap(err, "creating link file")
	}

	if err := os.Chmod(volumePath, 0755); err != nil {
		logger.Error("changing-volume-permissions-failed", err)
		return "", errorspkg.Wrap(err, "changing volume permissions")
	}
	return volumePath, nil
}

func (d *Driver) DestroyVolume(logger lager.Logger, id string) error {
	volumePath := filepath.Join(d.storePath, store.VolumesDirName, id)
	linkInfoPath := filepath.Join(d.storePath, LinksDirName, id)
	shortId, err := ioutil.ReadFile(linkInfoPath)
	if err != nil {
		return errorspkg.Wrapf(err, "getting volume symlink location from (%s)", linkInfoPath)
	}

	linkPath := filepath.Join(d.storePath, LinksDirName, string(shortId))
	if err := os.Remove(linkPath); err != nil {
		return errorspkg.Wrapf(err, "removing symlink %s", linkPath)
	}

	if err := os.Remove(linkInfoPath); err != nil {
		return errorspkg.Wrapf(err, "removing symlink information file %s", linkInfoPath)
	}

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

	baseVolumePaths, baseVolumeSize, err := d.getLowerDirs(logger, spec.BaseVolumeIDs)
	if err != nil {
		logger.Error("generating-lowerdir-paths-failed", err)
		return errorspkg.Wrap(err, "generating lowerdir paths failed")
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

	if err := os.Chdir(d.storePath); err != nil {
		return errorspkg.Wrap(err, "failed to change directory to the store path")
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

func (d *Driver) MoveVolume(logger lager.Logger, from, to string) error {
	logger.Debug("Moving volume from %s to %s\n", lager.Data{"from": from, "to": to})

	if err := os.Rename(from, to); err != nil {
		logger.Error("moving-volume-failed", err, lager.Data{"from": from, "to": to})
		return errorspkg.Wrap(err, "moving volume")
	}

	oldLinkFile := filepath.Join(d.storePath, LinksDirName, filepath.Base(from))
	shortId, err := ioutil.ReadFile(oldLinkFile)
	if err != nil {
		return errorspkg.Wrapf(err, "reading short id from %s", to)
	}

	newLinkFile := filepath.Join(d.storePath, LinksDirName, filepath.Base(to))
	if err := os.Rename(oldLinkFile, newLinkFile); err != nil {
		logger.Error("moving-link-file-failed", err, lager.Data{"from": oldLinkFile, "to": newLinkFile})
		return errorspkg.Wrap(err, "moving link file")
	}

	linkPath := filepath.Join(d.storePath, LinksDirName, string(shortId))
	if err := os.Remove(linkPath); err != nil {
		return errorspkg.Wrap(err, "removing symlink")
	}

	if err := os.Symlink(to, linkPath); err != nil {
		logger.Error("updating-volume-symlink-failed", err)
		return errorspkg.Wrap(err, "updating volume symlink")
	}

	return nil
}

func (d *Driver) getLowerDirs(logger lager.Logger, volumeIDs []string) ([]string, int64, error) {
	baseVolumePaths := []string{}
	var totalVolumeSize int64
	for i := len(volumeIDs) - 1; i >= 0; i-- {
		volumePath := filepath.Join(d.storePath, store.VolumesDirName, volumeIDs[i])

		if _, err := os.Stat(volumePath); os.IsNotExist(err) {
			logger.Error("base-volume-path-not-found", err)
			return nil, 0, errorspkg.Wrap(err, "base volume path does not exist")
		}

		volumeSize, err := d.duUsage(logger, volumePath)
		if err != nil {
			logger.Error("calculating-base-volume-size-failed", err)
			return nil, 0, errorspkg.Wrapf(err, "calculating base volume size %s", volumePath)
		}
		totalVolumeSize += volumeSize

		shortId, err := ioutil.ReadFile(filepath.Join(d.storePath, LinksDirName, volumeIDs[i]))
		if err != nil {
			return nil, 0, errorspkg.Wrapf(err, "reading short id  %s", volumePath)
		}

		baseVolumePaths = append(baseVolumePaths, filepath.Join(LinksDirName, string(shortId)))
	}
	return baseVolumePaths, totalVolumeSize, nil
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

func (d *Driver) generateID() (string, error) {
	sid, err := shortid.New(1, shortid.DefaultABC, 2342)
	if err != nil {
		return "", err
	}
	return sid.Generate()
}

func (d *Driver) createWhiteoutDevice(logger lager.Logger, storePath string, ownerUID, ownerGID int) error {
	whiteoutDevicePath := filepath.Join(storePath, WhiteoutDevice)
	if _, err := os.Stat(whiteoutDevicePath); os.IsNotExist(err) {
		if err := syscall.Mknod(whiteoutDevicePath, syscall.S_IFCHR, 0); err != nil {
			if err != nil && !os.IsExist(err) {
				logger.Error("creating-whiteout-device-failed", err, lager.Data{"path": whiteoutDevicePath})
				return errorspkg.Wrapf(err, "failed to create whiteout device %s", whiteoutDevicePath)
			}
		}

		if err := os.Chown(whiteoutDevicePath, ownerUID, ownerGID); err != nil {
			logger.Error("whiteout-device-ownership-change-failed", err, lager.Data{"target-uid": ownerUID, "target-gid": ownerGID})
			return errorspkg.Wrapf(err, "changing store owner to %d:%d for path %s", ownerUID, ownerGID, whiteoutDevicePath)
		}
	}
	return nil
}

func (d *Driver) validateWhiteoutDevice(storePath string) error {
	path := filepath.Join(storePath, WhiteoutDevice)

	stat, err := os.Stat(path)
	if err != nil && !os.IsExist(err) {
		return err
	}

	statT := stat.Sys().(*syscall.Stat_t)
	if statT.Rdev != 0 || (stat.Mode()&os.ModeCharDevice) != os.ModeCharDevice {
		return errorspkg.Errorf("the whiteout device file is not a valid device %s", path)
	}

	return nil
}

func (d *Driver) createLinksDirectory(logger lager.Logger, storePath string, ownerUID, ownerGID int) error {
	linksDir := filepath.Join(storePath, LinksDirName)
	if err := os.MkdirAll(linksDir, 0755); err != nil {
		logger.Error("mkdir-links-directory", err, lager.Data{"linksDir": linksDir})
		return errorspkg.Wrap(err, "creating links directory")
	}

	if err := os.Chmod(linksDir, 0755); err != nil {
		logger.Error("chmoding-links-directory", err, lager.Data{"linksDir": linksDir, "mode": "0755"})
		return errorspkg.Wrap(err, "chmoding links directory")
	}

	if err := os.Chown(linksDir, ownerUID, ownerGID); err != nil {
		logger.Error("chowning-links-directory", err, lager.Data{"linksDir": linksDir, "uid": ownerUID, "gid": ownerGID})
		return errorspkg.Wrap(err, "creating links directory")
	}

	return nil
}

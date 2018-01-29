package overlayxfs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/filesystems"
	quotapkg "code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs/quota"
	"code.cloudfoundry.org/grootfs/store/filesystems/spec"
	"code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
	"github.com/tscolari/lagregator"
	shortid "github.com/ventu-io/go-shortid"
)

const (
	UpperDir          = "diff"
	IDDir             = "projectids"
	WorkDir           = "workdir"
	RootfsDir         = "rootfs"
	imageInfoName     = "image_info"
	imageQuotaName    = "image_quota"
	WhiteoutDevice    = "whiteout_dev"
	LinksDirName      = "l"
	maxDestroyRetries = 5
	MinQuota          = 1024 * 256
)

func NewDriver(storePath, tardisBinPath string) *Driver {
	return &Driver{
		storePath:     storePath,
		tardisBinPath: tardisBinPath,
	}
}

type Driver struct {
	storePath     string
	tardisBinPath string
}

func (d *Driver) InitFilesystem(logger lager.Logger, filesystemPath, storePath string) error {
	logger = logger.Session("overlayxfs-init-filesystem", lager.Data{"filesystemPath": filesystemPath})
	logger.Debug("starting")
	defer logger.Debug("ending")

	if err := d.mountFilesystem(filesystemPath, storePath, "remount"); err != nil {
		if err := d.formatFilesystem(logger, filesystemPath); err != nil {
			return err
		}

		if err := d.mountFilesystem(filesystemPath, storePath, ""); err != nil {
			logger.Error("mounting-filesystem-failed", err, lager.Data{"filesystemPath": filesystemPath, "storePath": storePath})
			return errorspkg.Wrap(err, "Mounting filesystem")
		}
	}

	return nil
}

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

	linksDir := filepath.Join(path, LinksDirName)
	if err := d.createStoreDirectory(logger, linksDir, ownerUID, ownerGID); err != nil {
		logger.Error("creating-links-directory-failed", err)
		return errorspkg.Wrap(err, "Create links directory")
	}

	idsDir := filepath.Join(path, IDDir)
	if err := d.createStoreDirectory(logger, idsDir, ownerUID, ownerGID); err != nil {
		logger.Error("creating-ids-directory-failed", err)
		return errorspkg.Wrap(err, "Create ids directory")
	}

	return nil
}

func (d *Driver) ValidateFileSystem(logger lager.Logger, path string) error {
	logger = logger.Session("overlayxfs-validate-filesystem", lager.Data{"path": path})
	logger.Debug("starting")
	defer logger.Debug("ending")

	if err := filesystems.CheckFSPath(path, "xfs", "noatime", "nobarrier", "prjquota"); err != nil {
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

	shortId, err := d.generateShortishID()
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
	logger = logger.Session("overlayxfs-deleting-volume", lager.Data{"volumeID": id, "volumePath": volumePath, "linkInfoPath": linkInfoPath})
	logger.Info("starting")
	defer logger.Info("ending")

	if err := d.removeVolumeLink(linkInfoPath); err != nil {
		return err
	}

	volumeMetaFilePath := filesystems.VolumeMetaFilePath(d.storePath, id)
	if err := os.Remove(volumeMetaFilePath); err != nil {
		logger.Error("deleting-metadata-file-failed", err, lager.Data{"path": volumeMetaFilePath})
	}

	if err := os.RemoveAll(volumePath); err != nil {
		logger.Error(fmt.Sprintf("failed to destroy volume %s", volumePath), err)
		return errorspkg.Wrapf(err, "destroying volume (%s)", id)
	}
	return nil
}

func (d *Driver) removeVolumeLink(linkInfoPath string) error {
	shortId, err := ioutil.ReadFile(linkInfoPath)
	if err != nil && !os.IsNotExist(err) {
		return errorspkg.Wrapf(err, "getting volume symlink location from (%s)", linkInfoPath)
	}

	if err == nil || !os.IsNotExist(err) {
		linkPath := filepath.Join(d.storePath, LinksDirName, string(shortId))
		if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
			return errorspkg.Wrapf(err, "removing symlink %s", linkPath)
		}

		if err := os.Remove(linkInfoPath); err != nil && !os.IsNotExist(err) {
			return errorspkg.Wrapf(err, "removing symlink information file %s", linkInfoPath)
		}
	}

	return nil
}

func (d *Driver) Volumes(logger lager.Logger) ([]string, error) {
	logger = logger.Session("overlayxfs-list-volumes")
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

func (d *Driver) CreateImage(logger lager.Logger, spec image_cloner.ImageDriverSpec) (groot.MountInfo, error) {
	logger = logger.Session("overlayxfs-creating-image", lager.Data{"spec": spec})
	logger.Info("starting")
	defer logger.Info("ending")

	if _, err := os.Stat(spec.ImagePath); os.IsNotExist(err) {
		logger.Error("image-path-not-found", err)
		return groot.MountInfo{}, errorspkg.Wrap(err, "image path does not exist")
	}

	baseVolumePaths, baseVolumeSize, err := d.getLowerDirs(logger, spec.BaseVolumeIDs)
	if err != nil {
		logger.Error("generating-lowerdir-paths-failed", err)
		return groot.MountInfo{}, errorspkg.Wrap(err, "generating lowerdir paths failed")
	}

	if err := d.applyDiskLimit(logger, spec, baseVolumeSize); err != nil {
		return groot.MountInfo{}, errorspkg.Wrap(err, "applying disk limits")
	}

	upperDir := filepath.Join(spec.ImagePath, UpperDir)
	workDir := filepath.Join(spec.ImagePath, WorkDir)
	rootfsDir := filepath.Join(spec.ImagePath, RootfsDir)

	directories := map[string]string{
		"upperdir": upperDir,
		"workdir":  workDir,
		"rootfs":   rootfsDir,
	}

	if err := d.createImageDirectories(logger, directories); err != nil {
		return groot.MountInfo{}, err
	}

	if err := os.Chdir(d.storePath); err != nil {
		return groot.MountInfo{}, errorspkg.Wrap(err, "failed to change directory to the store path")
	}

	if spec.Mount {
		mountData := d.formatMountData(baseVolumePaths, workDir, upperDir, false)
		if err := d.mountImage(logger, rootfsDir, mountData); err != nil {
			return groot.MountInfo{}, err
		}
	}

	imageInfoFileName := filepath.Join(spec.ImagePath, imageInfoName)
	if err := ioutil.WriteFile(imageInfoFileName, []byte(strconv.FormatInt(baseVolumeSize, 10)), 0600); err != nil {
		return groot.MountInfo{}, errorspkg.Wrapf(err, "writing image info %s", imageInfoFileName)
	}

	return groot.MountInfo{
		Destination: "/",
		Source:      "overlay",
		Type:        "overlay",
		Options:     []string{d.formatMountData(baseVolumePaths, workDir, upperDir, true)},
	}, nil
}

func (d *Driver) MoveVolume(logger lager.Logger, from, to string) error {
	logger = logger.Session("overlayxfs-moving-volume", lager.Data{"from": from, "to": to})
	logger.Debug("starting")
	defer logger.Debug("ending")

	if _, err := os.Stat(from); os.IsNotExist(err) {
		return errorspkg.Wrap(err, "source volume doesn't exist")
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

	if err := os.Rename(from, to); err != nil {
		if os.IsExist(err) {
			return nil
		}

		logger.Error("moving-volume-failed", err, lager.Data{"from": from, "to": to})
		return errorspkg.Wrap(err, "moving volume")
	}

	return nil
}

func (d *Driver) HandleOpaqueWhiteouts(logger lager.Logger, id string, opaqueWhiteouts []string) error {
	if len(opaqueWhiteouts) == 0 {
		return nil
	}

	volumePath, err := d.VolumePath(logger, id)
	if err != nil {
		return err
	}

	args := make([]string, 0)

	for _, path := range opaqueWhiteouts {
		parentDir := filepath.Dir(filepath.Join(volumePath, path))
		args = append(args, "--opaque-path", parentDir)

		if err := cleanWhiteoutDir(parentDir); err != nil {
			return errorspkg.Wrapf(err, "clean without dir %s", parentDir)
		}
	}

	if output, err := d.runTardis(logger, append([]string{"handle-opqwhiteouts"}, args...)...); err != nil {
		logger.Error("handling-opaque-whiteouts-failed", err, lager.Data{"opaqueWhiteouts": opaqueWhiteouts})
		return errorspkg.Wrapf(err, "handle opaque whiteouts: %s", output.String())
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

func (d *Driver) WriteVolumeMeta(logger lager.Logger, id string, metadata base_image_puller.VolumeMeta) error {
	logger = logger.Session("overlayxfs-writing-volume-metadata", lager.Data{"volumeID": id})
	logger.Debug("starting")
	defer logger.Debug("ending")
	return filesystems.WriteVolumeMeta(logger, d.storePath, id, metadata)
}

func (d *Driver) formatFilesystem(logger lager.Logger, filesystemPath string) error {
	logger = logger.Session("formatting-filesystem")
	logger.Debug("starting")
	defer logger.Debug("ending")

	stdout := bytes.NewBuffer([]byte{})
	stderr := bytes.NewBuffer([]byte{})
	cmd := exec.Command("mkfs.xfs", "-f", filesystemPath)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		logger.Error("formatting-filesystem-failed", err, lager.Data{"cmd": cmd.Args, "stdout": stdout.String(), "stderr": stderr.String()})
		return errorspkg.Errorf("Formatting XFS filesystem: %s", err.Error())
	}

	return nil
}

func (d *Driver) mountFilesystem(source, destination, option string) error {
	allOpts := strings.Trim(fmt.Sprintf("%s,loop,pquota,noatime,nobarrier", option), ",")
	cmd := exec.Command("mount", "-o", allOpts, "-t", "xfs", source, destination)
	if output, err := cmd.CombinedOutput(); err != nil {
		return errorspkg.Errorf("%s: %s", err, string(output))
	}

	return nil
}

func (d *Driver) createImageDirectories(logger lager.Logger, directories map[string]string) error {
	for name, directory := range directories {
		if err := os.Mkdir(directory, 0755); err != nil {
			logger.Error(fmt.Sprintf("creating-%s-folder-failed", name), err)
			return errorspkg.Wrapf(err, "creating %s folder", name)
		}

		if err := os.Chmod(directory, 0755); err != nil {
			logger.Error(fmt.Sprintf("chmoding-%s-folder-failed", name), err)
			return errorspkg.Wrapf(err, "chmoding %s folder", name)
		}
	}

	return nil
}

func (d *Driver) formatMountData(lowerDirs []string, workDir, upperDir string, absolute bool) string {
	if absolute {
		for i, lowerDir := range lowerDirs {
			lowerDirs[i] = filepath.Join(d.storePath, lowerDir)
		}
	}

	lowerDirsOpt := strings.Join(lowerDirs, ":")
	return fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerDirsOpt, upperDir, workDir)
}

func (d *Driver) mountImage(logger lager.Logger, rootfsDir, mountData string) error {
	logger.Session("mounting-overlay-to-rootfs", lager.Data{"mountData": mountData, "rootfsDir": rootfsDir})
	logger.Info("starting")
	defer logger.Info("ending")

	if err := syscall.Mount("overlay", rootfsDir, "overlay", 0, mountData); err != nil {
		logger.Error("failed", err, lager.Data{"mountData": mountData, "rootfsDir": rootfsDir})
		return errorspkg.Wrap(err, "mounting overlay")
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

		volumeSize, err := d.VolumeSize(logger, volumeIDs[i])
		if err != nil {
			logger.Error("calculating-base-volume-size-failed", err, lager.Data{"volumeID": volumeIDs[i]})
			return nil, 0, errorspkg.Wrapf(err, "calculating base volume size for volume %s", volumeIDs[i])
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

	projectID, err := quotapkg.GetProjectID(logger, imagePath)
	if err != nil {
		logger.Error("fetching-project-id-failed", err)
		logger.Info("skipping-project-id-folder-removal")
	}

	if err := ensureImageDestroyed(logger, imagePath); err != nil {
		logger.Error("removing-image-path-failed", err)
		return errorspkg.Wrap(err, "deleting rootfs folder")
	}

	if projectID != 0 {
		if err := os.RemoveAll(filepath.Join(d.storePath, IDDir, strconv.Itoa(int(projectID)))); err != nil {
			logger.Error("removing-project-id-folder-failed", err)
		}
	}

	return nil
}

func (d *Driver) FetchStats(logger lager.Logger, imagePath string) (groot.VolumeStats, error) {
	logger = logger.Session("overlayxfs-fetching-stats", lager.Data{"imagePath": imagePath})
	logger.Debug("starting")
	defer logger.Debug("ending")

	output, err := d.runTardis(logger, "stats", "--volume-path", imagePath)
	if err != nil {
		logger.Error("fetching-stats-failed", err, lager.Data{"imagePath": imagePath})
		return groot.VolumeStats{}, errorspkg.Wrapf(err, "fetch stats: %s", output.String())
	}

	stats := groot.VolumeStats{}
	if err := json.Unmarshal(output.Bytes(), &stats); err != nil {
		logger.Error("unmarshaling-json-stats-failed", err, lager.Data{"stats": output.String(), "imagePath": imagePath})
		return groot.VolumeStats{}, errorspkg.Wrapf(err, "fetch stats: %s", output.String())
	}

	return stats, nil
}

func (d *Driver) Marshal(logger lager.Logger) ([]byte, error) {
	driverSpec := spec.DriverSpec{
		Type:           "overlay-xfs",
		StorePath:      d.storePath,
		FsBinaryPath:   "",
		MkfsBinaryPath: "",
		SuidBinaryPath: d.tardisBinPath,
	}

	return json.Marshal(driverSpec)
}

func (d *Driver) VolumeSize(logger lager.Logger, id string) (int64, error) {
	logger = logger.Session("overlayxfs-volume-size", lager.Data{"volumeID": id})
	logger.Debug("starting")
	defer logger.Debug("ending")

	return filesystems.VolumeSize(logger, d.storePath, id)
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

func (d *Driver) createStoreDirectory(logger lager.Logger, path string, ownerUID, ownerGID int) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		logger.Error("mkdir-path", err, lager.Data{"path": path})
		return errorspkg.Wrap(err, "creating directory")
	}

	if err := os.Chmod(path, 0755); err != nil {
		logger.Error("chmoding-path", err, lager.Data{"path": path, "mode": "0755"})
		return errorspkg.Wrap(err, "chmoding directory")
	}

	if err := os.Chown(path, ownerUID, ownerGID); err != nil {
		logger.Error("chowning-path", err, lager.Data{"path": path, "uid": ownerUID, "gid": ownerGID})
		return errorspkg.Wrap(err, "creating directory")
	}

	return nil
}

func (d *Driver) runTardis(logger lager.Logger, args ...string) (*bytes.Buffer, error) {
	logger = logger.Session("run-tardis", lager.Data{"path": d.tardisBinPath, "args": args})
	logger.Debug("starting")
	defer logger.Debug("ending")

	if !d.tardisInPath() {
		return nil, errorspkg.New("tardis was not found in the $PATH")
	}

	if !d.hasSUID() && os.Geteuid() != 0 {
		return nil, errorspkg.New("missing the setuid bit on tardis")
	}

	cmd := exec.Command(d.tardisBinPath, args...)
	stdoutBuffer := bytes.NewBuffer([]byte{})
	relogger := lagregator.NewRelogger(logger)
	cmd.Stdout = io.MultiWriter(stdoutBuffer, relogger)
	cmd.Stderr = relogger

	err := cmd.Run()

	if err != nil {
		logger.Error("tardis-failed", err)
		return nil, errorspkg.Wrapf(err, " %s", strings.TrimSpace(stdoutBuffer.String()))
	}

	return stdoutBuffer, nil
}

func (d *Driver) tardisInPath() bool {
	if _, err := exec.LookPath(d.tardisBinPath); err != nil {
		return false
	}
	return true
}

func (d *Driver) hasSUID() bool {
	path, err := exec.LookPath(d.tardisBinPath)
	if err != nil {
		return false
	}
	// If LookPath succeeds Stat cannot fail
	stats, _ := os.Stat(path)
	if (stats.Mode() & os.ModeSetuid) == 0 {
		return false
	}
	return true
}

func (d *Driver) generateShortishID() (string, error) {
	id, err := shortid.Generate()
	return id + strconv.Itoa(os.Getpid()), err
}

func (d *Driver) applyDiskLimit(logger lager.Logger, spec image_cloner.ImageDriverSpec, volumeSize int64) error {
	logger = logger.Session("applying-quotas", lager.Data{"spec": spec})
	logger.Debug("starting")
	defer logger.Debug("ending")

	if spec.DiskLimit == 0 {
		logger.Debug("no-need-for-quotas")
		return nil
	}

	diskLimit := spec.DiskLimit
	if spec.ExclusiveDiskLimit {
		logger.Debug("applying-exclusive-quotas")
	} else {
		logger.Debug("applying-inclusive-quotas")
		diskLimit -= volumeSize
		if diskLimit < 0 {
			err := errorspkg.New("disk limit is smaller than volume size")
			logger.Error("applying-inclusive-quota-failed", err, lager.Data{"imagePath": spec.ImagePath})
			return err
		}
	}

	if diskLimit < MinQuota {
		logger.Debug("overwriting-disk-quota", lager.Data{"oldLimit": diskLimit, "newLimit": MinQuota})
		diskLimit = MinQuota
	}

	diskLimitString := strconv.FormatInt(diskLimit, 10)

	if output, err := d.runTardis(logger, "limit", "--disk-limit-bytes", diskLimitString, "--image-path", spec.ImagePath); err != nil {
		logger.Error("applying-quota-failed", err, lager.Data{"diskLimit": diskLimit, "imagePath": spec.ImagePath})
		return errorspkg.Wrapf(err, "apply disk limit: %s", output.String())
	}

	if err := ioutil.WriteFile(filepath.Join(spec.ImagePath, imageQuotaName), []byte(diskLimitString), 0600); err != nil {
		logger.Error("writing-image-quota-failed", err)
		return errorspkg.Wrap(err, "writing image quota")
	}
	return nil
}

func ensureImageDestroyed(logger lager.Logger, imagePath string) error {
	if err := syscall.Unmount(filepath.Join(imagePath, RootfsDir), 0); err != nil {
		logger.Info("unmount image path failed", lager.Data{"path": imagePath, "error": err})
	}
	return os.RemoveAll(imagePath)
}

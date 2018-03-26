// +build linux,cgo

// This code was copied from https://github.com/docker/docker/blob/v1.13.1/daemon/graphdriver/quota/projectquota.go
// License: Apache License

//
// projectquota.go - implements XFS project quota controls
// for setting quota limits on a newly created directory.
// It currently supports the legacy XFS specific ioctls.
//
// TODO: use generic quota control ioctl FS_IOC_FS{GET,SET}XATTR
//       for both xfs/ext4 for kernel version >= v4.5
//

package quota

/*
#include <stdlib.h>
#include <dirent.h>
#include <linux/fs.h>
#include <linux/quota.h>
#include <linux/dqblk_xfs.h>

#ifndef FS_XFLAG_PROJINHERIT
struct fsxattr {
	__u32		fsx_xflags;
	__u32		fsx_extsize;
	__u32		fsx_nextents;
	__u32		fsx_projid;
	unsigned char	fsx_pad[12];
};
#define FS_XFLAG_PROJINHERIT	0x00000200
#endif
#ifndef FS_IOC_FSGETXATTR
#define FS_IOC_FSGETXATTR		_IOR ('X', 31, struct fsxattr)
#endif
#ifndef FS_IOC_FSSETXATTR
#define FS_IOC_FSSETXATTR		_IOW ('X', 32, struct fsxattr)
#endif

#ifndef PRJQUOTA
#define PRJQUOTA	2
#endif
#ifndef XFS_PROJ_QUOTA
#define XFS_PROJ_QUOTA	2
#endif
#ifndef Q_XSETPQLIM
#define Q_XSETPQLIM QCMD(Q_XSETQLIM, PRJQUOTA)
#endif
#ifndef Q_XGETPQUOTA
#define Q_XGETPQUOTA QCMD(Q_XGETQUOTA, PRJQUOTA)
#endif
*/
import "C"
import (
	"os"
	"path"
	"path/filepath"
	"unsafe"

	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func Get(logger lager.Logger, path string) (Quota, error) {
	logger = logger.Session("get-quota", lager.Data{"path": path})
	logger.Debug("starting")
	defer logger.Debug("ending")

	var quota Quota
	projectID, err := GetProjectID(logger, path)
	if err != nil {
		logger.Error("getting-project-id-failed", err)
		return quota, err
	}

	if projectID == 0 {
		return Quota{Size: 0, BCount: 0}, nil
	}

	storeDevicePath, err := getStoreDevicePath(path)
	if err != nil {
		logger.Error("ensuring-backing-fs-device-failed", err)
		return quota, err
	}

	//
	// get the quota limit for the container's project id
	//
	var d C.fs_disk_quota_t

	var cs = C.CString(storeDevicePath)
	defer C.free(unsafe.Pointer(cs))

	_, _, errno := unix.Syscall6(unix.SYS_QUOTACTL, C.Q_XGETPQUOTA,
		uintptr(unsafe.Pointer(cs)), uintptr(C.__u32(projectID)),
		uintptr(unsafe.Pointer(&d)), 0, 0)
	if errno != 0 {
		logger.Error("getting-quota-for-project-id-failed", errno)
		return quota, errors.Errorf("getting quota limit for projid %d: %v",
			projectID, errno.Error())
	}

	quota.Size = uint64(d.d_blk_hardlimit) * 512
	quota.BCount = uint64(d.d_bcount) * 512
	return quota, nil
}

func Set(logger lager.Logger, projectID uint32, path string, quotaSize uint64) error {
	logger = logger.Session("set-quota", lager.Data{"projectID": projectID})
	logger.Debug("starting")
	defer logger.Debug("ending")

	if err := setProjectID(projectID, path); err != nil {
		logger.Error("setting-project-id-failed", err)
		return err
	}

	storeDevicePath, err := getStoreDevicePath(path)
	if err != nil {
		logger.Error("ensuring-backing-fs-device-failed", err)
		return err
	}

	var d C.fs_disk_quota_t
	d.d_version = C.FS_DQUOT_VERSION
	d.d_id = C.__u32(projectID)
	d.d_flags = C.XFS_PROJ_QUOTA

	d.d_fieldmask = C.FS_DQ_BHARD | C.FS_DQ_BSOFT
	d.d_blk_hardlimit = C.__u64(quotaSize / 512)
	d.d_blk_softlimit = d.d_blk_hardlimit

	var cs = C.CString(storeDevicePath)
	defer C.free(unsafe.Pointer(cs))

	_, _, errno := unix.Syscall6(unix.SYS_QUOTACTL, C.Q_XSETPQLIM,
		uintptr(unsafe.Pointer(cs)), uintptr(d.d_id),
		uintptr(unsafe.Pointer(&d)), 0, 0)
	if errno != 0 {
		logger.Error("setting-quota-to-project-id-failed", errno)
		return errors.Errorf("setting quota limit for projid %d: %v",
			projectID, errno.Error())
	}

	return nil
}

func GetProjectID(logger lager.Logger, path string) (uint32, error) {
	logger = logger.Session("get-projectid", lager.Data{"path": path})
	logger.Debug("starting")
	defer logger.Debug("ending")

	dir, err := openDir(path)
	if err != nil {
		return 0, err
	}
	defer closeDir(dir)

	fsx, err := getXattr(dir)
	if err != nil {
		return 0, errors.Wrapf(err, "getting extended attributes for %s", path)
	}

	projectID := uint32(fsx.fsx_projid)
	logger.Debug("project-id-acquired", lager.Data{"projectID": projectID})

	return uint32(fsx.fsx_projid), nil
}

func setProjectID(projectID uint32, path string) error {
	dir, err := openDir(path)
	if err != nil {
		return err
	}
	defer closeDir(dir)

	fsx, err := getXattr(dir)
	if err != nil {
		return errors.Wrapf(err, "getting extended attributes for %s", path)
	}

	fsx.fsx_projid = C.__u32(projectID)
	fsx.fsx_xflags |= C.FS_XFLAG_PROJINHERIT

	if err := setXattr(dir, fsx); err != nil {
		return errors.Wrapf(err, "setting extended attributes for %s", path)
	}

	return nil
}

func getXattr(dir *C.DIR) (C.struct_fsxattr, error) {
	var fsx C.struct_fsxattr

	_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.FS_IOC_FSGETXATTR,
		uintptr(unsafe.Pointer(&fsx)))
	if errno != 0 {
		return fsx, errno
	}

	return fsx, nil
}

func setXattr(dir *C.DIR, fsx C.struct_fsxattr) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.FS_IOC_FSSETXATTR,
		uintptr(unsafe.Pointer(&fsx)))
	if errno != 0 {
		return errno
	}

	return nil
}

func getStoreDevicePath(imagePath string) (string, error) {
	basePath := filepath.Dir(filepath.Dir(imagePath))

	storeDevicePath := path.Join(basePath, "storeDevice")
	if _, err := os.Stat(storeDevicePath); err == nil {
		return storeDevicePath, nil
	}

	var stat unix.Stat_t
	if err := unix.Stat(basePath, &stat); err != nil {
		return "", err
	}

	// mknod will create a new block device with the same major and minor numbers as returned by stat (but with a different path)
	if err := unix.Mknod(storeDevicePath, unix.S_IFBLK|0600, int(stat.Dev)); err != nil && !os.IsExist(err) {
		return "", errors.Errorf("creating backing fs block device %s: %v", storeDevicePath, err)
	}

	return storeDevicePath, nil
}

func openDir(path string) (*C.DIR, error) {
	Cpath := C.CString(path)
	defer free(Cpath)

	dir := C.opendir(Cpath)
	if dir == nil {
		return nil, errors.Errorf("opening directory: %s", path)
	}
	return dir, nil
}

func closeDir(dir *C.DIR) {
	if dir != nil {
		C.closedir(dir)
	}
}

func getDirFd(dir *C.DIR) uintptr {
	return uintptr(C.dirfd(dir))
}

func free(p *C.char) {
	C.free(unsafe.Pointer(p))
}

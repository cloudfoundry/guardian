package unpacker // import "code.cloudfoundry.org/grootfs/base_image_puller/unpacker"

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/system"
	"github.com/pkg/errors"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/lager/v3"
)

const (
	defaultDirectoryFileMode = 0755
	defaultDirectoryUid      = 0
	defaultDirectoryGid      = 0
)

//go:generate counterfeiter . WhiteoutHandler
type WhiteoutHandler interface {
	RemoveWhiteout(path string) error
}

type TarUnpacker struct {
	whiteoutHandler WhiteoutHandler
	idTranslator    IDTranslator
}

func NewTarUnpacker(whiteoutHandler WhiteoutHandler, idTranslator IDTranslator) *TarUnpacker {
	return &TarUnpacker{
		whiteoutHandler: whiteoutHandler,
		idTranslator:    idTranslator,
	}
}

func (u *TarUnpacker) Unpack(logger lager.Logger, spec base_image_puller.UnpackSpec) (base_image_puller.UnpackOutput, error) {
	logger = logger.Session("unpacking-with-tar", lager.Data{"spec": spec})
	logger.Info("starting")
	defer logger.Info("ending")

	if err := safeMkdir(spec.TargetPath, 0755); err != nil {
		return base_image_puller.UnpackOutput{}, err
	}

	tarReader := tar.NewReader(spec.Stream)
	opaqueWhiteouts := []string{}
	var totalBytesUnpacked int64
	for {
		tarHeader, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return base_image_puller.UnpackOutput{}, err
		}

		entryPath := filepath.Join(spec.BaseDirectory, tarHeader.Name)
		entryTargetPath := filepath.Join(spec.TargetPath, entryPath)

		if strings.Contains(tarHeader.Name, ".wh..wh..opq") {
			opaqueWhiteouts = append(opaqueWhiteouts, entryPath)
			continue
		}

		if strings.Contains(tarHeader.Name, ".wh.") {
			if err := u.whiteoutHandler.RemoveWhiteout(entryTargetPath); err != nil {
				return base_image_puller.UnpackOutput{}, err
			}
			continue
		}

		entrySize, err := u.handleEntry(entryTargetPath, tarReader, tarHeader, spec)
		if err != nil {
			return base_image_puller.UnpackOutput{}, err
		}

		totalBytesUnpacked += entrySize
	}

	return base_image_puller.UnpackOutput{
		BytesWritten:    totalBytesUnpacked,
		OpaqueWhiteouts: opaqueWhiteouts,
	}, nil
}

func (u *TarUnpacker) handleEntry(entryPath string, tarReader *tar.Reader, tarHeader *tar.Header, spec base_image_puller.UnpackSpec) (entrySize int64, err error) {
	if err := u.ensureParentDir(entryPath); err != nil {
		return 0, err
	}

	switch tarHeader.Typeflag {
	case tar.TypeBlock, tar.TypeChar:
		// ignore devices
		return 0, nil

	case tar.TypeLink:
		if err = u.createLink(entryPath, tarHeader, spec); err != nil {
			return 0, err
		}

	case tar.TypeSymlink:
		if err = u.createSymlink(entryPath, tarHeader, spec); err != nil {
			return 0, err
		}

	case tar.TypeDir:
		if err = u.createDirectory(entryPath, tarHeader, spec); err != nil {
			return 0, err
		}

	case tar.TypeReg, tar.TypeRegA:
		if entrySize, err = u.createRegularFile(entryPath, tarHeader, tarReader, spec); err != nil {
			return 0, err
		}
	}

	return entrySize, nil
}

func (u *TarUnpacker) createDirectory(path string, tarHeader *tar.Header, spec base_image_puller.UnpackSpec) error {
	if _, err := os.Stat(path); err != nil {
		if err = os.Mkdir(path, tarHeader.FileInfo().Mode()); err != nil {
			newErr := errors.Wrapf(err, "creating directory `%s`", path)

			if os.IsPermission(err) {
				dirName := filepath.Dir(tarHeader.Name)
				return errors.Errorf("'/%s' does not give write permission to its owner. This image can only be unpacked using uid and gid mappings, or by running as root.", dirName)
			}

			return newErr
		}
	}

	uid := u.idTranslator.TranslateUID(tarHeader.Uid)
	gid := u.idTranslator.TranslateGID(tarHeader.Gid)
	if err := os.Chown(path, uid, gid); err != nil {
		return errors.Wrapf(err, "chowning directory %d:%d `%s`", uid, gid, path)
	}

	// we need to explicitly apply perms because mkdir is subject to umask
	if err := os.Chmod(path, tarHeader.FileInfo().Mode()); err != nil {
		return errors.Wrapf(err, "chmoding directory `%s`", path)
	}

	if err := changeModTime(path, tarHeader.ModTime); err != nil {
		return errors.Wrapf(err, "setting the modtime for directory %s", path)
	}

	return nil
}

func (u *TarUnpacker) createSymlink(path string, tarHeader *tar.Header, spec base_image_puller.UnpackSpec) error {
	if _, err := os.Lstat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return errors.Wrapf(err, "removing file `%s`", path)
		}
	}

	if err := os.Symlink(tarHeader.Linkname, path); err != nil {
		return errors.Wrapf(err, "create symlink `%s` -> `%s`", tarHeader.Linkname, path)
	}

	if err := changeModTime(path, tarHeader.ModTime); err != nil {
		return errors.Wrapf(err, "setting the modtime for the symlink `%s`", path)
	}

	uid := u.idTranslator.TranslateUID(tarHeader.Uid)
	gid := u.idTranslator.TranslateGID(tarHeader.Gid)
	if err := os.Lchown(path, uid, gid); err != nil {
		return errors.Wrapf(err, "chowning link %d:%d `%s`", uid, gid, path)
	}

	return nil
}

func (u *TarUnpacker) createLink(path string, tarHeader *tar.Header, spec base_image_puller.UnpackSpec) error {
	return os.Link(filepath.Join(spec.TargetPath, spec.BaseDirectory, tarHeader.Linkname), path)
}

func (u *TarUnpacker) ensureParentDir(childPath string) error {
	parentDirPath := filepath.Dir(childPath)

	if _, err := os.Stat(parentDirPath); err != nil {
		if err := u.ensureParentDir(parentDirPath); err != nil {
			return err
		}

		if err := os.Mkdir(parentDirPath, defaultDirectoryFileMode); err != nil {
			return errors.Wrapf(err, "creating parent dir `%s`", parentDirPath)
		}

		if err := os.Chmod(parentDirPath, defaultDirectoryFileMode); err != nil {
			return err
		}

		uid := u.idTranslator.TranslateUID(defaultDirectoryUid)
		gid := u.idTranslator.TranslateGID(defaultDirectoryGid)
		return os.Chown(parentDirPath, uid, gid)
	}

	return nil
}

func (u *TarUnpacker) createRegularFile(path string, tarHeader *tar.Header, tarReader *tar.Reader, spec base_image_puller.UnpackSpec) (int64, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, tarHeader.FileInfo().Mode())
	if err != nil {
		newErr := errors.Wrapf(err, "creating file `%s`", path)

		if os.IsPermission(err) {
			dirName := filepath.Dir(tarHeader.Name)
			return 0, errors.Errorf("'/%s' does not give write permission to its owner. This image can only be unpacked using uid and gid mappings, or by running as root.", dirName)
		}

		return 0, newErr
	}

	fileSize, err := io.Copy(file, tarReader)
	if err != nil {
		_ = file.Close()
		return 0, errors.Wrapf(err, "writing to file `%s`", path)
	}

	if err := file.Close(); err != nil {
		return 0, errors.Wrapf(err, "closing file `%s`", path)
	}

	uid := u.idTranslator.TranslateUID(tarHeader.Uid)
	gid := u.idTranslator.TranslateGID(tarHeader.Gid)
	if err := os.Chown(path, uid, gid); err != nil {
		return 0, errors.Wrapf(err, "chowning file %d:%d `%s`", uid, gid, path)
	}

	// we need to explicitly apply perms because mkdir is subject to umask
	if err := os.Chmod(path, tarHeader.FileInfo().Mode()); err != nil {
		return 0, errors.Wrapf(err, "chmoding file `%s`", path)
	}

	if err := changeModTime(path, tarHeader.ModTime); err != nil {
		return 0, errors.Wrapf(err, "setting the modtime for file `%s`", path)
	}

	for key, value := range tarHeader.PAXRecords {
		if strings.HasPrefix(key, "SCHILY.xattr") {
			xattrName := strings.TrimPrefix(key, "SCHILY.xattr.")
			err = system.Lsetxattr(path, xattrName, []byte(value), 0)
			if err != nil {
				return 0, errors.Wrapf(err, "setting xattr `%s` for file `%s`", xattrName, path)
			}
		}
	}

	return fileSize, nil
}

func safeMkdir(path string, perm os.FileMode) error {
	if _, err := os.Stat(path); err != nil {
		if err := os.Mkdir(path, perm); err != nil {
			return errors.Wrapf(err, "making destination directory `%s`", path)
		}
	}
	return nil
}

package unpacker // import "code.cloudfoundry.org/grootfs/base_image_puller/unpacker"

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	errorspkg "github.com/pkg/errors"

	"github.com/docker/docker/pkg/reexec"
	"github.com/urfave/cli"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/lager"
	"github.com/containers/storage/pkg/system"
)

func init() {
	var fail = func(logger lager.Logger, message string, err error) {
		logger.Error(message, err)
		fmt.Println(err.Error())
		os.Exit(1)
	}

	reexec.Register("unpack", func() {
		cli.ErrWriter = os.Stdout
		logger := lager.NewLogger("unpack")
		logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.DEBUG))

		rootFSPath := os.Args[1]
		unpackStrategyJson := os.Args[2]

		var unpackStrategy UnpackStrategy
		err := json.Unmarshal([]byte(unpackStrategyJson), &unpackStrategy)
		if err != nil {
			fail(logger, "unmarshal-unpack-strategy-failed", err)
		}

		unpacker := NewTarUnpacker(unpackStrategy)
		if err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
			Stream:     os.Stdin,
			TargetPath: rootFSPath,
		}); err != nil {
			fail(logger, "unpacking-failed", err)
		}
	})
}

type UnpackStrategy struct {
	Name               string
	WhiteoutDevicePath string
}

type TarUnpacker struct {
	whiteoutHandler whiteoutHandler
}

func NewTarUnpacker(unpackStrategy UnpackStrategy) *TarUnpacker {
	var whiteoutHandler whiteoutHandler

	switch unpackStrategy.Name {
	case "overlay-xfs":
		whiteoutHandler = &overlayWhiteoutHandler{
			whiteoutDevicePath: unpackStrategy.WhiteoutDevicePath,
		}
	default:
		whiteoutHandler = &defaultWhiteoutHandler{}
	}

	return &TarUnpacker{
		whiteoutHandler: whiteoutHandler,
	}
}

type whiteoutHandler interface {
	removeWhiteout(path string) error
	removeOpaqueWhiteouts(paths []string) error
}

type overlayWhiteoutHandler struct {
	whiteoutDevicePath string
}

func (h *overlayWhiteoutHandler) removeWhiteout(path string) error {
	toBeDeletedPath := strings.Replace(path, ".wh.", "", 1)
	if err := os.RemoveAll(toBeDeletedPath); err != nil {
		return errorspkg.Wrap(err, "deleting  file")
	}

	if err := os.Link(h.whiteoutDevicePath, toBeDeletedPath); err != nil {
		return errorspkg.Wrapf(err, "failed to create whiteout node: %s", toBeDeletedPath)
	}

	return nil
}

func (*overlayWhiteoutHandler) removeOpaqueWhiteouts(paths []string) error {
	for _, path := range paths {
		parentDir := filepath.Dir(path)
		if err := system.Lsetxattr(parentDir, "trusted.overlay.opaque", []byte("y"), 0); err != nil {
			return err
		}

		if err := cleanWhiteoutDir(parentDir); err != nil {
			return err
		}
	}

	return nil
}

type defaultWhiteoutHandler struct{}

func (*defaultWhiteoutHandler) removeWhiteout(path string) error {
	toBeDeletedPath := strings.Replace(path, ".wh.", "", 1)
	if err := os.RemoveAll(toBeDeletedPath); err != nil {
		return errorspkg.Wrap(err, "deleting whiteout file")
	}

	return nil
}

func (*defaultWhiteoutHandler) removeOpaqueWhiteouts(paths []string) error {
	for _, p := range paths {
		parentDir := path.Dir(p)
		if err := cleanWhiteoutDir(parentDir); err != nil {
			return err
		}
	}

	return nil
}

func (u *TarUnpacker) Unpack(logger lager.Logger, spec base_image_puller.UnpackSpec) error {
	logger = logger.Session("unpacking-with-tar", lager.Data{"spec": spec})
	logger.Info("starting")
	defer logger.Info("ending")

	if _, err := os.Stat(spec.TargetPath); err != nil {
		if err := os.Mkdir(spec.TargetPath, 0755); err != nil {
			return errorspkg.Wrapf(err, "making destination directory `%s`", spec.TargetPath)
		}
	}

	tarReader := tar.NewReader(spec.Stream)
	opaqueWhiteouts := []string{}
	for {
		tarHeader, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		entryPath := filepath.Join(spec.TargetPath, tarHeader.Name)

		if strings.Contains(entryPath, ".wh..wh..opq") {
			opaqueWhiteouts = append(opaqueWhiteouts, entryPath)
			continue
		}
		if strings.Contains(entryPath, ".wh.") {
			if err := u.whiteoutHandler.removeWhiteout(entryPath); err != nil {
				return err
			}
			continue
		}

		if err := u.handleEntry(spec.TargetPath, entryPath, tarReader, tarHeader); err != nil {
			return err
		}
	}

	return u.whiteoutHandler.removeOpaqueWhiteouts(opaqueWhiteouts)
}

func (u *TarUnpacker) handleEntry(targetPath, entryPath string, tarReader *tar.Reader, tarHeader *tar.Header) error {
	switch tarHeader.Typeflag {
	case tar.TypeBlock, tar.TypeChar:
		// ignore devices
		return nil

	case tar.TypeLink:
		if err := u.createLink(targetPath, entryPath, tarHeader); err != nil {
			return err
		}

	case tar.TypeSymlink:
		if err := u.createSymlink(entryPath, tarHeader); err != nil {
			return err
		}

	case tar.TypeDir:
		if err := u.createDirectory(entryPath, tarHeader); err != nil {
			return err
		}

	case tar.TypeReg, tar.TypeRegA:
		if err := u.createRegularFile(entryPath, tarHeader, tarReader); err != nil {
			return err
		}
	}

	return nil
}

func (u *TarUnpacker) createDirectory(path string, tarHeader *tar.Header) error {
	if _, err := os.Stat(path); err != nil {
		if err = os.Mkdir(path, tarHeader.FileInfo().Mode()); err != nil {
			newErr := errorspkg.Wrapf(err, "creating directory `%s`", path)

			if os.IsPermission(err) {
				dirName := filepath.Dir(tarHeader.Name)
				return errorspkg.Errorf("'/%s' does not give write permission to its owner. This image can only be unpacked using uid and gid mappings, or by running as root.", dirName)
			}

			return newErr
		}
	}

	if os.Getuid() == 0 {
		if err := os.Chown(path, tarHeader.Uid, tarHeader.Gid); err != nil {
			return errorspkg.Wrapf(err, "chowning directory %d:%d `%s`", tarHeader.Uid, tarHeader.Gid, path)
		}
	}

	// we need to explicitly apply perms because mkdir is subject to umask
	if err := os.Chmod(path, tarHeader.FileInfo().Mode()); err != nil {
		return errorspkg.Wrapf(err, "chmoding directory `%s`", path)
	}

	if err := changeModTime(path, tarHeader.ModTime); err != nil {
		return errorspkg.Wrapf(err, "setting the modtime for directory `%s`: %s", path)
	}

	return nil
}

func (u *TarUnpacker) createSymlink(path string, tarHeader *tar.Header) error {
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return errorspkg.Wrapf(err, "removing file `%s`", path)
		}
	}

	if err := os.Symlink(tarHeader.Linkname, path); err != nil {
		return errorspkg.Wrapf(err, "create symlink `%s` -> `%s`", tarHeader.Linkname, path)
	}

	if err := changeModTime(path, tarHeader.ModTime); err != nil {
		return errorspkg.Wrapf(err, "setting the modtime for the symlink `%s`", path)
	}

	return nil
}

func (u *TarUnpacker) createLink(targetPath, path string, tarHeader *tar.Header) error {
	return os.Link(filepath.Join(targetPath, tarHeader.Linkname), path)
}

func (u *TarUnpacker) createRegularFile(path string, tarHeader *tar.Header, tarReader *tar.Reader) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, tarHeader.FileInfo().Mode())
	if err != nil {
		newErr := errorspkg.Wrapf(err, "creating file `%s`", path)

		if os.IsPermission(err) {
			dirName := filepath.Dir(tarHeader.Name)
			return errorspkg.Errorf("'/%s' does not give write permission to its owner. This image can only be unpacked using uid and gid mappings, or by running as root.", dirName)
		}

		return newErr
	}

	_, err = io.Copy(file, tarReader)
	if err != nil {
		file.Close()
		return errorspkg.Wrapf(err, "writing to file `%s`", path)
	}

	if err := file.Close(); err != nil {
		return errorspkg.Wrapf(err, "closing file `%s`", path)
	}

	if os.Getuid() == 0 {
		if err := os.Chown(path, tarHeader.Uid, tarHeader.Gid); err != nil {
			return errorspkg.Wrapf(err, "chowning file %d:%d `%s`", tarHeader.Uid, tarHeader.Gid, path)
		}
	}

	// we need to explicitly apply perms because mkdir is subject to umask
	if err := os.Chmod(path, tarHeader.FileInfo().Mode()); err != nil {
		return errorspkg.Wrapf(err, "chmoding file `%s`", path)
	}

	if err := changeModTime(path, tarHeader.ModTime); err != nil {
		return errorspkg.Wrapf(err, "setting the modtime for file `%s`", path)
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

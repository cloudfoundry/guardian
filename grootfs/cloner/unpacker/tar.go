package unpacker

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/grootfs/cloner"
	"code.cloudfoundry.org/lager"
)

type TarUnpacker struct {
}

func NewTarUnpacker() *TarUnpacker {
	return &TarUnpacker{}
}

func (u *TarUnpacker) Unpack(logger lager.Logger, spec cloner.UnpackSpec) error {
	logger = logger.Session("unpacking-with-tar", lager.Data{"spec": spec})
	logger.Debug("start")
	defer logger.Debug("end")

	if _, err := os.Stat(spec.TargetPath); err != nil {
		if err := os.Mkdir(spec.TargetPath, 0755); err != nil {
			return fmt.Errorf("making destination directory `%s`: %s", spec.TargetPath, err)
		}
	}

	if err := u.unTar(spec); err != nil {
		return fmt.Errorf("failed to untar: %s", err)
	}

	return nil
}

func (u *TarUnpacker) unTar(spec cloner.UnpackSpec) error {
	tarReader := tar.NewReader(spec.Stream)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		path := filepath.Join(spec.TargetPath, header.Name)
		info := header.FileInfo()

		switch u.fileType(path, info) {
		case "DEVICE":
			continue
		case "DIRECTORY":
			if err := u.createDirectory(path, header); err != nil {
				return err
			}
		case "WHITEOUT":
			if err := u.removeWhiteout(path); err != nil {
				return err
			}
		case "REGULAR_FILE":
			if err := u.createRegularFile(path, header, tarReader); err != nil {
				return err
			}
		}
	}

	return nil
}

func (u *TarUnpacker) fileType(path string, info os.FileInfo) string {
	if info.IsDir() {
		return "DIRECTORY"
	}

	if strings.Contains(path, "/dev/") {
		return "DEVICE"
	}

	if strings.Contains(path, ".wh.") {
		return "WHITEOUT"
	}

	return "REGULAR_FILE"
}

func (u *TarUnpacker) createDirectory(path string, tarHeader *tar.Header) error {
	if _, err := os.Stat(path); err != nil {
		if err = os.Mkdir(path, os.FileMode(tarHeader.Mode)); err != nil {
			return fmt.Errorf("creating directory `%s`: %s", path, err)
		}
	}

	if os.Getuid() == 0 {
		if err := os.Chown(path, tarHeader.Uid, tarHeader.Gid); err != nil {
			return fmt.Errorf("chowning file %d:%d `%s`: %s", tarHeader.Uid, tarHeader.Gid, path, err)
		}
	}

	return nil
}

func (u *TarUnpacker) removeWhiteout(path string) error {
	tbdPath := strings.Replace(path, ".wh.", "", 1)
	if err := os.RemoveAll(tbdPath); err != nil {
		return fmt.Errorf("deleting whiteout file: %s", err)
	}

	return nil
}

func (u *TarUnpacker) createRegularFile(path string, tarHeader *tar.Header, tarReader *tar.Reader) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(tarHeader.Mode))
	if err != nil {
		return fmt.Errorf("creating file `%s`: %s", path, err)
	}
	defer file.Close()

	if os.Getuid() == 0 {
		if err := os.Chown(path, tarHeader.Uid, tarHeader.Gid); err != nil {
			return fmt.Errorf("chowning file %d:%d `%s`: %s", tarHeader.Uid, tarHeader.Gid, path, err)
		}
	}

	_, err = io.Copy(file, tarReader)
	if err != nil {
		return fmt.Errorf("writing to file `%s`: %s", path, err)
	}

	return nil
}

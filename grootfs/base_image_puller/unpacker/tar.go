package unpacker // import "code.cloudfoundry.org/grootfs/base_image_puller/unpacker"

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/lager"
)

type TarUnpacker struct {
}

func NewTarUnpacker() *TarUnpacker {
	return &TarUnpacker{}
}

func (u *TarUnpacker) Unpack(logger lager.Logger, spec base_image_puller.UnpackSpec) error {
	logger = logger.Session("unpacking-with-tar", lager.Data{"spec": spec})
	logger.Info("start")
	defer logger.Info("end")

	if _, err := os.Stat(spec.TargetPath); err != nil {
		if err := os.Mkdir(spec.TargetPath, 0755); err != nil {
			return fmt.Errorf("making destination directory `%s`: %s", spec.TargetPath, err)
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
			if err := u.removeWhiteout(entryPath); err != nil {
				return err
			}
			continue
		}

		if err := u.handleEntry(spec.TargetPath, entryPath, tarReader, tarHeader); err != nil {
			return err
		}
	}

	return u.removeOpaqueWhiteouts(opaqueWhiteouts)
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
			return fmt.Errorf("creating directory `%s`: %s", path, err)
		}
	}

	if os.Getuid() == 0 {
		if err := os.Chown(path, tarHeader.Uid, tarHeader.Gid); err != nil {
			return fmt.Errorf("chowning directory %d:%d `%s`: %s", tarHeader.Uid, tarHeader.Gid, path, err)
		}
	}

	// we need to explicitly apply perms because mkdir is subject to umask
	if err := os.Chmod(path, tarHeader.FileInfo().Mode()); err != nil {
		return fmt.Errorf("chmoding directory `%s`: %s", path, err)
	}

	if err := os.Chtimes(path, time.Now(), tarHeader.ModTime); err != nil {
		return fmt.Errorf("setting the modtime for directory `%s`: %s", path, err)
	}

	return nil
}

func (u *TarUnpacker) removeOpaqueWhiteouts(paths []string) error {
	for _, p := range paths {
		folder := path.Dir(p)
		contents, err := ioutil.ReadDir(folder)
		if err != nil {
			return fmt.Errorf("reading whiteout directory: %s", err)
		}

		for _, content := range contents {
			if err := os.RemoveAll(path.Join(folder, content.Name())); err != nil {
				return fmt.Errorf("cleaning up whiteout directory: %s", err)
			}
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

func (u *TarUnpacker) createSymlink(path string, tarHeader *tar.Header) error {
	return os.Symlink(tarHeader.Linkname, path)
}

func (u *TarUnpacker) createLink(targetPath, path string, tarHeader *tar.Header) error {
	return os.Link(filepath.Join(targetPath, tarHeader.Linkname), path)
}

func (u *TarUnpacker) createRegularFile(path string, tarHeader *tar.Header, tarReader *tar.Reader) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, tarHeader.FileInfo().Mode())
	if err != nil {
		return fmt.Errorf("creating file `%s`: %s", path, err)
	}

	_, err = io.Copy(file, tarReader)
	if err != nil {
		file.Close()
		return fmt.Errorf("writing to file `%s`: %s", path, err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("closing file `%s`: %s", path, err)
	}

	if os.Getuid() == 0 {
		if err := os.Chown(path, tarHeader.Uid, tarHeader.Gid); err != nil {
			return fmt.Errorf("chowning file %d:%d `%s`: %s", tarHeader.Uid, tarHeader.Gid, path, err)
		}
	}

	// we need to explicitly apply perms because mkdir is subject to umask
	if err := os.Chmod(path, tarHeader.FileInfo().Mode()); err != nil {
		return fmt.Errorf("chmoding file `%s`: %s", path, err)
	}

	if err := os.Chtimes(path, time.Now(), tarHeader.ModTime); err != nil {
		return fmt.Errorf("setting the modtime for file `%s`: %s", path, err)
	}

	return nil
}

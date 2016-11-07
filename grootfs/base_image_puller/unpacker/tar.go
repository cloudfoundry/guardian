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

	if err := u.unTar(spec); err != nil {
		return fmt.Errorf("failed to untar: %s", err)
	}

	return nil
}

func (u *TarUnpacker) unTar(spec base_image_puller.UnpackSpec) error {
	tarReader := tar.NewReader(spec.Stream)
	opaqueWhiteouts := []string{}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		path := filepath.Join(spec.TargetPath, header.Name)

		if strings.Contains(path, ".wh..wh..opq") {
			opaqueWhiteouts = append(opaqueWhiteouts, path)
			continue
		}

		if strings.Contains(path, ".wh.") {
			if err := u.removeWhiteout(path); err != nil {
				return err
			}
			continue
		}

		switch header.Typeflag {
		case tar.TypeBlock, tar.TypeChar:
			continue

		case tar.TypeLink:
			if err := u.createLink(spec.TargetPath, path, header); err != nil {
				return err
			}

		case tar.TypeSymlink:
			if err := u.createSymlink(path, header); err != nil {
				return err
			}

		case tar.TypeDir:
			if err := u.createDirectory(path, header); err != nil {
				return err
			}

		case tar.TypeReg, tar.TypeRegA:
			if err := u.createRegularFile(path, header, tarReader); err != nil {
				return err
			}
		}
	}

	return u.removeOpaqueWhiteouts(opaqueWhiteouts)
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

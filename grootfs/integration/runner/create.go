package runner

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	yaml "gopkg.in/yaml.v2"

	errorspkg "github.com/pkg/errors"

	"github.com/onsi/gomega/gexec"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
)

func (r Runner) StartCreate(spec groot.CreateSpec) (*gexec.Session, error) {
	if !r.skipInitStore {
		if err := r.initStore(); err != nil {
			return nil, err
		}
	}
	args := r.makeCreateArgs(spec)
	return r.StartSubcommand("create", args...)
}

func (r Runner) Create(spec groot.CreateSpec) (groot.ImageInfo, error) {
	if !r.skipInitStore {
		if err := r.initStore(); err != nil {
			return groot.ImageInfo{}, err
		}
	}

	args := r.makeCreateArgs(spec)
	output, err := r.RunSubcommand("create", args...)
	if err != nil {
		return groot.ImageInfo{}, err
	}

	imageInfo := groot.ImageInfo{}

	if r.Json || spec.Json {
		json.Unmarshal([]byte(output), &imageInfo)
		imageInfo.Path = filepath.Dir(imageInfo.Rootfs)
	} else {
		imageInfo.Path = output
		imageInfo.Rootfs = filepath.Join(imageInfo.Path, "rootfs")
	}

	return imageInfo, nil
}

func (r Runner) EnsureMounted(image groot.ImageInfo) error {
	if image.Mount != nil {
		return syscall.Mount(image.Mount.Source, image.Mount.Destination, image.Mount.Type, 0, image.Mount.Options[0])
	}

	return nil
}

func (r Runner) makeCreateArgs(spec groot.CreateSpec) []string {
	args := []string{}
	for _, mapping := range spec.UIDMappings {
		args = append(args, "--uid-mapping",
			fmt.Sprintf("%d:%d:%d", mapping.NamespaceID, mapping.HostID, mapping.Size),
		)
	}
	for _, mapping := range spec.GIDMappings {
		args = append(args, "--gid-mapping",
			fmt.Sprintf("%d:%d:%d", mapping.NamespaceID, mapping.HostID, mapping.Size),
		)
	}

	if r.CleanOnCreate || r.NoCleanOnCreate {
		if r.CleanOnCreate {
			args = append(args, "--with-clean")
		}
		if r.NoCleanOnCreate {
			args = append(args, "--without-clean")
		}
	} else {
		if spec.CleanOnCreate {
			args = append(args, "--with-clean")
		} else {
			args = append(args, "--without-clean")
		}
	}

	if r.Json || r.NoJson {
		if r.Json {
			args = append(args, "--json")
		}
		if r.NoJson {
			args = append(args, "--no-json")
		}
	} else {
		if spec.Json {
			args = append(args, "--json")
		}
	}

	if spec.Mount {
		args = append(args, "--with-mount")
	} else {
		args = append(args, "--without-mount")
	}

	if r.InsecureRegistry != "" {
		args = append(args, "--insecure-registry", r.InsecureRegistry)
	}

	if r.RegistryUsername != "" {
		args = append(args, "--username", r.RegistryUsername)
	}

	if r.RegistryPassword != "" {
		args = append(args, "--password", r.RegistryPassword)
	}

	if spec.DiskLimit != 0 {
		args = append(args, "--disk-limit-size-bytes",
			strconv.FormatInt(spec.DiskLimit, 10),
		)
		if spec.ExcludeBaseImageFromQuota {
			args = append(args, "--exclude-image-from-quota")
		}
	}

	if spec.BaseImage != "" {
		args = append(args, spec.BaseImage)
	}

	if spec.ID != "" {
		args = append(args, spec.ID)
	}

	return args
}

func (r Runner) initStore() error {
	GrootfsTestUid, _ := strconv.Atoi(os.Getenv("GROOTFS_TEST_UID"))
	GrootfsTestGid, _ := strconv.Atoi(os.Getenv("GROOTFS_TEST_GID"))

	storePath := r.StorePath
	if r.StorePath == "" {
		configBytes, err := ioutil.ReadFile(r.ConfigPath)

		if err != nil {
			return err
		}
		cfg := config.Config{}
		err = yaml.Unmarshal(configBytes, &cfg)
		if err != nil {
			return err
		}
		storePath = cfg.StorePath
	}

	whiteoutDevicePath := filepath.Join(storePath, overlayxfs.WhiteoutDevice)

	if _, err := os.Stat(storePath); os.IsNotExist(err) {
		os.MkdirAll(storePath, 0700)
	}

	if err := os.Chown(storePath, GrootfsTestUid, GrootfsTestGid); err != nil {
		return errorspkg.Wrapf(err, "changing store owner to %d:%d for path %s", GrootfsTestUid, GrootfsTestGid, storePath)
	}

	if r.Driver == "overlay-xfs" {
		if _, err := os.Stat(whiteoutDevicePath); os.IsNotExist(err) {
			if err := syscall.Mknod(whiteoutDevicePath, syscall.S_IFCHR, 0); err != nil {
				if err != nil && !os.IsExist(err) {
					return errorspkg.Wrapf(err, "failed to create whiteout device %s", whiteoutDevicePath)
				}
			}

			if err := os.Chown(whiteoutDevicePath, GrootfsTestUid, GrootfsTestGid); err != nil {
				return errorspkg.Wrapf(err, "changing store owner to %d:%d for path %s", GrootfsTestUid, GrootfsTestGid, whiteoutDevicePath)
			}
		}
	}
	return nil
}

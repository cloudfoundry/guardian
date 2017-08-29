package runner

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/onsi/gomega/gexec"

	"code.cloudfoundry.org/grootfs/groot"
)

func (r Runner) StartCreate(spec groot.CreateSpec) (*gexec.Session, error) {
	if !r.skipInitStore {
		if err := r.initStoreAsRoot(); err != nil {
			return nil, err
		}
	}
	args := r.makeCreateArgs(spec)
	return r.StartSubcommand("create", args...)
}

func (r Runner) Create(spec groot.CreateSpec) (groot.ImageInfo, error) {
	if len(spec.UIDMappings) > 0 || len(spec.GIDMappings) > 0 {
		return groot.ImageInfo{}, errors.New("Mappings cannot be applied to Create. Set them on init-store.")
	}

	if !r.skipInitStore {
		if err := r.initStoreAsRoot(); err != nil {
			return groot.ImageInfo{}, err
		}
	}

	args := r.makeCreateArgs(spec)
	output, err := r.RunSubcommand("create", args...)
	if err != nil {
		return groot.ImageInfo{}, err
	}

	imageInfo := groot.ImageInfo{}
	_ = json.Unmarshal([]byte(output), &imageInfo)
	imageInfo.Path = filepath.Dir(imageInfo.Rootfs)

	return imageInfo, nil
}

func (r Runner) EnsureMounted(image groot.ImageInfo) error {
	if len(image.Mounts) != 0 {
		for _, mountPoint := range image.Mounts {
			return syscall.Mount(mountPoint.Source, mountPoint.Destination, mountPoint.Type, 0, mountPoint.Options[0])
		}
	}

	return nil
}

func (r Runner) initStoreAsRoot() error {
	spec := InitSpec{}

	if r.SysCredential.Uid != 0 {
		spec.UIDMappings = defaultIdMapping(r.SysCredential.Uid)
		spec.GIDMappings = defaultIdMapping(r.SysCredential.Gid)
	}

	if err := r.RunningAsUser(0, 0).InitStore(spec); err != nil {
		return err
	}

	return nil
}

func defaultIdMapping(hostId uint32) []groot.IDMappingSpec {
	return []groot.IDMappingSpec{
		groot.IDMappingSpec{
			HostID:      int(hostId),
			NamespaceID: 0,
			Size:        1,
		},
		{HostID: 100000, NamespaceID: 1, Size: 65000},
	}
}

func (r Runner) makeCreateArgs(spec groot.CreateSpec) []string {
	args := []string{}

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

	if r.SkipLayerValidation {
		args = append(args, "--skip-layer-validation")
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

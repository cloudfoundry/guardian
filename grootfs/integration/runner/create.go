package runner

import (
	"fmt"
	"path/filepath"
	"strconv"

	"code.cloudfoundry.org/grootfs/groot"
)

func (r Runner) Create(spec groot.CreateSpec) (groot.Image, error) {
	args := r.makeCreateArgs(spec)
	imagePath, err := r.RunSubcommand("create", args...)
	if err != nil {
		return groot.Image{}, err
	}

	return groot.Image{
		Path:       imagePath,
		RootFSPath: filepath.Join(imagePath, "rootfs"),
	}, nil
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
	if spec.CleanUpStore {
		args = append(args, "--clean")
	}
	if spec.DiskLimit != 0 {
		args = append(args, "--disk-limit-size-bytes",
			strconv.FormatInt(spec.DiskLimit, 10),
		)
		if spec.ExcludeBaseImageFromQuota {
			args = append(args, "--exclude-image-from-quota")
		}
	}
	args = append(args, spec.BaseImage)
	args = append(args, spec.ID)

	return args
}

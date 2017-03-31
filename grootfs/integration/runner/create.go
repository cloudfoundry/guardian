package runner

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/onsi/gomega/gexec"

	"code.cloudfoundry.org/grootfs/groot"
)

func (r Runner) StartCreate(spec groot.CreateSpec) (*gexec.Session, error) {
	args := r.makeCreateArgs(spec)
	return r.StartSubcommand("create", args...)
}

func (r Runner) Create(spec groot.CreateSpec, extraArgs ...string) (groot.Image, error) {
	output, err := r.create(spec)
	if err != nil {
		return groot.Image{}, err
	}

	var path string
	imageInfo := groot.ImageInfo{}

	if r.Json || spec.Json {
		json.Unmarshal([]byte(output), &imageInfo)
		path = filepath.Dir(imageInfo.Rootfs)
	} else {
		path = output
	}

	return groot.Image{
		Path:       path,
		RootFSPath: filepath.Join(path, "rootfs"),
		ImageInfo:  imageInfo,
	}, nil
}

func (r Runner) create(spec groot.CreateSpec) (string, error) {
	args := r.makeCreateArgs(spec)
	image, err := r.RunSubcommand("create", args...)
	if err != nil {
		return "", err
	}

	return image, nil
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

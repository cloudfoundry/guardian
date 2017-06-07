package bundlerules

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type EtcMounts struct {
	Chowner Chowner
}

//go:generate counterfeiter . Chowner
type Chowner interface {
	Chown(path string, uid, gid int) error
}

func (r EtcMounts) Apply(bndl goci.Bndl, spec gardener.DesiredContainerSpec, containerDir string) (goci.Bndl, error) {
	containerRootHostUID := mappingForContainerRoot(bndl.UIDMappings())
	containerRootHostGID := mappingForContainerRoot(bndl.GIDMappings())
	if err := r.createFile(filepath.Join(containerDir, "hosts"), int(containerRootHostUID), containerRootHostGID); err != nil {
		return goci.Bndl{}, err
	}
	if err := r.createFile(filepath.Join(containerDir, "resolv.conf"), int(containerRootHostUID), containerRootHostGID); err != nil {
		return goci.Bndl{}, err
	}

	mounts := []specs.Mount{
		{
			Destination: "/etc/hosts",
			Source:      filepath.Join(containerDir, "hosts"),
			Type:        "bind",
			Options:     []string{"bind"},
		},
		{
			Destination: "/etc/resolv.conf",
			Source:      filepath.Join(containerDir, "resolv.conf"),
			Type:        "bind",
			Options:     []string{"bind"},
		},
	}
	return bndl.WithMounts(mounts...), nil
}

func mappingForContainerRoot(mappings []specs.LinuxIDMapping) int {
	for _, mapping := range mappings {
		if mapping.ContainerID == 0 {
			return int(mapping.HostID)
		}
	}

	return 0
}

func (r EtcMounts) createFile(path string, containerRootHostUID, containerRootHostGID int) error {
	if err := touchFile(path); err != nil {
		return err
	}
	if err := r.Chowner.Chown(path, containerRootHostUID, containerRootHostGID); err != nil {
		wrappedErr := fmt.Errorf("error chowning %s: %s", filepath.Base(path), err)
		return wrappedErr
	}

	return nil
}

func touchFile(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	return file.Close()
}

type OSChowner struct{}

func (*OSChowner) Chown(path string, uid, gid int) error {
	return os.Chown(path, uid, gid)
}

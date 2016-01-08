package runrunc

import (
	"fmt"
	"path/filepath"

	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/rundmc/depot"
	"github.com/opencontainers/runc/libcontainer/user"
	"github.com/pivotal-golang/lager"
)

type UidGidGetter struct {
	BundleGetter     BundleGetter
	Logger           lager.Logger
	BundleLoader     depot.BundleLoader
	PasswdFileParser PasswdFileParser
}

//go:generate counterfeiter . BundleGetter
type BundleGetter interface {
	GetBundle(log lager.Logger, bundleloader depot.BundleLoader, containerID string) (*goci.Bndl, error)
}

type PasswdFileParser func(passwdPath string) ([]user.User, error)

//go:generate counterfeiter . PasswdFileParserObject
type PasswdFileParserObject interface {
	ParsePasswdFile(passwdPath string) ([]user.User, error)
}

//TODO define a type for UID and GID
// On success, returns the UID, the GID, and nil in that order.
func (i *UidGidGetter) GetIDs(containerId string, userName string) (uint32, uint32, error) {
	containerBundle, err := i.BundleGetter.GetBundle(i.Logger, i.BundleLoader, containerId)
	if err != nil {
		return 0, 0, err
	}

	if containerBundle == nil || containerBundle.Spec.Spec.Root.Path == "" {
		return 0, 0, fmt.Errorf("No root filesystem path found for container id '%s'", containerId)
	}
	rootFsPath := containerBundle.Spec.Spec.Root.Path

	users, err := i.PasswdFileParser(filepath.Join(rootFsPath, "/etc/passwd"))
	if err != nil {
		return 0, 0, err
	}

	if userName == "" {
		return 0, 0, nil
	}

	for i := 0; i < len(users); i++ {
		if users[i].Name == userName {
			return uint32(users[i].Uid), uint32(users[i].Gid), nil
		}
	}

	return 0, 0, fmt.Errorf("No matching user name '%s'", userName)
}

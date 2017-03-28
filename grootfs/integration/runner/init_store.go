package runner

import (
	"fmt"

	"code.cloudfoundry.org/grootfs/store/manager"
)

func (r Runner) InitStore(spec manager.InitSpec) error {
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

	_, err := r.RunSubcommand("init-store", args...)
	return err
}

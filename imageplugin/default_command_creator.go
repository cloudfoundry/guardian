package imageplugin

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	specs "github.com/opencontainers/runtime-spec/specs-go"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden-shed/rootfs_spec"
	"code.cloudfoundry.org/lager"
)

type DefaultCommandCreator struct {
	BinPath   string
	ExtraArgs []string
}

func (cc *DefaultCommandCreator) CreateCommand(log lager.Logger, handle string, spec rootfs_spec.Spec) (*exec.Cmd, error) {
	args := append(cc.ExtraArgs, "create")

	if spec.QuotaSize > 0 {
		args = append(args, "--disk-limit-size-bytes", strconv.FormatInt(spec.QuotaSize, 10))

		if spec.QuotaScope == garden.DiskLimitScopeExclusive {
			args = append(args, "--exclude-image-from-quota")
		}
	}

	if spec.Username != "" {
		args = append(args, "--username", spec.Username)
	}

	if spec.Password != "" {
		args = append(args, "--password", spec.Password)
	}

	rootfs := strings.Replace(spec.RootFS.String(), "#", ":", 1)

	args = append(args, rootfs, handle)
	return exec.Command(cc.BinPath, args...), nil
}

func (cc *DefaultCommandCreator) DestroyCommand(log lager.Logger, handle string) *exec.Cmd {
	return exec.Command(cc.BinPath, append(cc.ExtraArgs, "delete", handle)...)
}

func (cc *DefaultCommandCreator) MetricsCommand(log lager.Logger, handle string) *exec.Cmd {
	return exec.Command(cc.BinPath, append(cc.ExtraArgs, "stats", handle)...)
}

func stringifyMapping(mapping specs.LinuxIDMapping) string {
	return fmt.Sprintf("%d:%d:%d", mapping.ContainerID, mapping.HostID, mapping.Size)
}

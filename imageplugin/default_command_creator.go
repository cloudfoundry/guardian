package imageplugin

import (
	"os/exec"
	"strconv"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/lager"
)

type DefaultCommandCreator struct {
	BinPath   string
	ExtraArgs []string
}

func (cc *DefaultCommandCreator) CreateCommand(log lager.Logger, handle string, spec gardener.RootfsSpec) (*exec.Cmd, error) {
	extraArgs := make([]string, len(cc.ExtraArgs))
	copy(extraArgs, cc.ExtraArgs)
	args := append(extraArgs, "create")

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
	return exec.Command(cc.BinPath, append(clone(cc.ExtraArgs), "delete", handle)...)
}

func (cc *DefaultCommandCreator) MetricsCommand(log lager.Logger, handle string) *exec.Cmd {
	return exec.Command(cc.BinPath, append(clone(cc.ExtraArgs), "stats", handle)...)
}

// append is not thread safe when operating on shared memory, such as cc.ExtraArgs. We therefore clone the slice and then append additional valies.
// See https://medium.com/@cep21/gos-append-is-not-always-thread-safe-a3034db7975
func clone(values []string) []string {
	clone := make([]string, 0, len(values))
	return append(clone, values...)
}

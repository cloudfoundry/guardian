package processes

import (
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

//go:generate counterfeiter . EnvDeterminer
type EnvDeterminer interface {
	EnvFor(bndl goci.Bndl, spec runrunc.ProcessSpec) []string
}

type EnvFunc func(bndl goci.Bndl, spec runrunc.ProcessSpec) []string

func (fn EnvFunc) EnvFor(bndl goci.Bndl, spec runrunc.ProcessSpec) []string {
	return fn(bndl, spec)
}

type ProcBuilder struct {
	envDeterminer  EnvDeterminer
	nonRootMaxCaps []string
}

func NewBuilder(envDeterminer EnvDeterminer, nonRootMaxCaps []string) *ProcBuilder {
	return &ProcBuilder{
		envDeterminer:  envDeterminer,
		nonRootMaxCaps: nonRootMaxCaps,
	}
}

func (p *ProcBuilder) BuildProcess(bndl goci.Bndl, spec runrunc.ProcessSpec) *specs.Process {
	return &specs.Process{
		Args:        append([]string{spec.Path}, spec.Args...),
		ConsoleSize: console(spec),
		Env:         p.envDeterminer.EnvFor(bndl, spec),
		User: specs.User{
			UID:            uint32(spec.ContainerUID),
			GID:            uint32(spec.ContainerGID),
			AdditionalGids: []uint32{},
			Username:       spec.User,
		},
		Cwd:             spec.Dir,
		Capabilities:    p.capabilities(bndl, spec.ContainerUID),
		Rlimits:         toRlimits(spec.Limits),
		Terminal:        spec.TTY != nil,
		ApparmorProfile: bndl.Process().ApparmorProfile,
	}
}

func (p *ProcBuilder) capabilities(bndl goci.Bndl, containerUID int) *specs.LinuxCapabilities {
	capsToSet := bndl.Capabilities()
	if containerUID != 0 {
		capsToSet = intersect(capsToSet, p.nonRootMaxCaps)
	}

	// TODO centralize knowledge of garden -> runc capability schema translation
	if len(capsToSet) > 0 {
		return &specs.LinuxCapabilities{
			Bounding:    capsToSet,
			Inheritable: capsToSet,
			Permitted:   capsToSet,
		}
	}

	return nil
}

func console(spec runrunc.ProcessSpec) *specs.Box {
	consoleBox := &specs.Box{
		Width:  80,
		Height: 24,
	}
	if spec.TTY != nil && spec.TTY.WindowSize != nil {
		consoleBox.Width = uint(spec.TTY.WindowSize.Columns)
		consoleBox.Height = uint(spec.TTY.WindowSize.Rows)
	}
	return consoleBox
}

func intersect(l1 []string, l2 []string) (result []string) {
	for _, a := range l1 {
		for _, b := range l2 {
			if a == b {
				result = append(result, a)
			}
		}
	}

	return result
}

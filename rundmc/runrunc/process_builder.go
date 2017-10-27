package runrunc

import (
	"code.cloudfoundry.org/guardian/rundmc/goci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type ProcBuilder struct {
	envDeterminer  EnvDeterminer
	nonRootMaxCaps []string
}

func NewProcessBuilder(envDeterminer EnvDeterminer, nonRootMaxCaps []string) *ProcBuilder {
	return &ProcBuilder{
		envDeterminer:  envDeterminer,
		nonRootMaxCaps: nonRootMaxCaps,
	}
}

func (p *ProcBuilder) BuildProcess(bndl goci.Bndl, spec ProcessSpec) *PreparedSpec {
	return &PreparedSpec{
		ContainerRootHostUID: containerRootHostID(bndl.Spec.Linux.UIDMappings),
		ContainerRootHostGID: containerRootHostID(bndl.Spec.Linux.GIDMappings),
		Process: specs.Process{
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
		},
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

func console(spec ProcessSpec) *specs.Box {
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

func containerRootHostID(mappings []specs.LinuxIDMapping) uint32 {
	for _, mapping := range mappings {
		if mapping.ContainerID == 0 {
			return mapping.HostID
		}
	}
	return 0
}

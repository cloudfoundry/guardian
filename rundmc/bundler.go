package rundmc

import (
	"fmt"
	"os"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/goci/specs"
	"github.com/cloudfoundry-incubator/guardian/gardener"
)

//go:generate counterfeiter . BundlerRule
type BundlerRule interface {
	Apply(bndle *goci.Bndl, spec gardener.DesiredContainerSpec) *goci.Bndl
}

type BundleTemplate struct {
	Rules []BundlerRule
}

func (b BundleTemplate) Generate(spec gardener.DesiredContainerSpec) *goci.Bndl {
	var bndl *goci.Bndl

	for _, rule := range b.Rules {
		bndl = rule.Apply(bndl, spec)
	}

	return bndl
}

type BaseTemplateRule struct {
	PrivilegedBase   *goci.Bndl
	UnprivilegedBase *goci.Bndl
}

func (r BaseTemplateRule) Apply(bndl *goci.Bndl, spec gardener.DesiredContainerSpec) *goci.Bndl {
	if spec.Privileged {
		return r.PrivilegedBase
	} else {
		return r.UnprivilegedBase
	}
}

type RootFSRule struct{}

func (r RootFSRule) Apply(bndl *goci.Bndl, spec gardener.DesiredContainerSpec) *goci.Bndl {
	return bndl.WithRootFS(spec.RootFSPath)
}

type NetworkHookRule struct {
	LogFilePattern string
}

func (r NetworkHookRule) Apply(bndl *goci.Bndl, spec gardener.DesiredContainerSpec) *goci.Bndl {
	env := []string{fmt.Sprintf(
		"GARDEN_LOG_FILE="+r.LogFilePattern, spec.Handle),
		"PATH=" + os.Getenv("PATH"),
	}

	return bndl.WithPrestartHooks(specs.Hook{
		Env:  env,
		Path: spec.NetworkHooks.Prestart.Path,
		Args: spec.NetworkHooks.Prestart.Args,
	}).WithPoststopHooks(specs.Hook{
		Env:  env,
		Path: spec.NetworkHooks.Poststop.Path,
		Args: spec.NetworkHooks.Poststop.Args,
	})
}

type BindMountsRule struct {
}

func (b BindMountsRule) Apply(bndl *goci.Bndl, spec gardener.DesiredContainerSpec) *goci.Bndl {
	var mounts []goci.Mount
	for _, m := range spec.BindMounts {
		modeOpt := "ro"
		if m.Mode == garden.BindMountModeRW {
			modeOpt = "rw"
		}

		mounts = append(mounts, goci.Mount{
			Name:        m.DstPath,
			Destination: m.DstPath,
			Source:      m.SrcPath,
			Type:        "bind",
			Options:     []string{"bind", modeOpt},
		})
	}

	return bndl.WithMounts(mounts...)
}

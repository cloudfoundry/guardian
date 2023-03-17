package peas

import (
	"errors"
	"path/filepath"
	"strconv"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/users"
	"code.cloudfoundry.org/lager/v3"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

//go:generate counterfeiter . ProcessPidGetter
type ProcessPidGetter interface {
	GetPid(log lager.Logger, handle string) (int, error)
	GetPeaPid(log lager.Logger, handle, peaID string) (int, error)
}

//go:generate counterfeiter . BundleLoader
type BundleLoader interface {
	Load(log lager.Logger, handle string) (goci.Bndl, error)
}

type PeaUsernameResolver struct {
	PidGetter     ProcessPidGetter
	PeaCreator    rundmc.PeaCreator
	UserLookupper users.UserLookupper
	BundleLoader  BundleLoader
}

func (r *PeaUsernameResolver) ResolveUser(log lager.Logger, handle string, image garden.ImageRef, username string) (int, int, error) {
	log = log.Session("resolve-user", lager.Data{"handle": handle, "image": image, "username": username})
	log.Info("resolve-user-start")
	defer log.Info("resolve-user-ended")

	bndl, err := r.BundleLoader.Load(log, handle)
	if err != nil {
		return -1, -1, err
	}

	gardenInitBindMount, err := findGardenInitBindMount(bndl.Spec.Mounts)
	if err != nil {
		return -1, -1, err
	}

	resolveUserPea, err := r.PeaCreator.CreatePea(
		log, garden.ProcessSpec{
			Path:       bndl.Spec.Process.Args[0],
			User:       "0:0",
			BindMounts: []garden.BindMount{gardenInitBindMount},
			Image:      image,
		}, garden.ProcessIO{}, handle,
	)
	if err != nil {
		return -1, -1, err
	}

	defer func() {
		if killErr := resolveUserPea.Signal(garden.SignalKill); killErr != nil {
			log.Error("resolve-user-pea-signal-failed", killErr)
			return
		}
		if _, waitErr := resolveUserPea.Wait(); err != nil {
			log.Error("resolve-user-pea-wait-failed", waitErr)
		}
	}()

	resolveUserPeaPid, err := r.PidGetter.GetPeaPid(log, handle, resolveUserPea.ID())
	if err != nil {
		return -1, -1, err
	}

	lookedupUser, err := r.UserLookupper.Lookup(filepath.Join("/proc", strconv.Itoa(resolveUserPeaPid), "root"), username)
	if err != nil {
		return -1, -1, err
	}

	log.Debug("username-resolved", lager.Data{"username": username, "uid": lookedupUser.Uid, "gid": lookedupUser.Gid})
	return lookedupUser.Uid, lookedupUser.Gid, nil
}

func findGardenInitBindMount(mounts []specs.Mount) (garden.BindMount, error) {
	for _, m := range mounts {
		if m.Type == "bind" && m.Destination == "/tmp/garden-init" {
			return garden.BindMount{
				SrcPath: m.Source,
				DstPath: m.Destination,
			}, nil
		}
	}

	return garden.BindMount{}, errors.New("Could not find bind mount to /tmp/garden-init")
}

package runrunc

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/idmapper"
	"code.cloudfoundry.org/lager"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type execPreparer struct {
	bundleLoader  BundleLoader
	users         UserLookupper
	envDeterminer EnvDeterminer
	mkdirer       Mkdirer
	runningAsRoot func() bool

	nonRootMaxCaps []string
}

func NewExecPreparer(bundleLoader BundleLoader, userlookup UserLookupper, envDeterminer EnvDeterminer, mkdirer Mkdirer, nonRootMaxCaps []string, runningAsRootFunc func() bool) ExecPreparer {
	return &execPreparer{
		bundleLoader:   bundleLoader,
		users:          userlookup,
		envDeterminer:  envDeterminer,
		mkdirer:        mkdirer,
		nonRootMaxCaps: nonRootMaxCaps,
		runningAsRoot:  runningAsRootFunc,
	}
}

func (r *execPreparer) Prepare(log lager.Logger, bundlePath string, spec garden.ProcessSpec) (*PreparedSpec, error) {
	log = log.Session("prepare")

	log.Info("start")
	defer log.Info("finished")

	bndl, err := r.bundleLoader.Load(bundlePath)
	if err != nil {
		log.Error("load-bundle-failed", err)
		return nil, err
	}

	pidBytes, err := ioutil.ReadFile(filepath.Join(bundlePath, "pidfile"))
	if err != nil {
		log.Error("read-pidfile-failed", err)
		return nil, err
	}
	pid := string(pidBytes)

	rootFSPathFile := filepath.Join("/proc", pid, "root")
	u, err := r.lookupUser(bndl, rootFSPathFile, spec.User)
	if err != nil {
		log.Error("lookup-user-failed", err)
		return nil, err
	}

	cwd := u.home
	if spec.Dir != "" {
		cwd = spec.Dir
	}

	if err := r.ensureDirExists(rootFSPathFile, cwd, u.hostUid, u.hostGid); err != nil {
		log.Error("ensure-dir-failed", err)
		return nil, err
	}

	capsToSet := bndl.Capabilities()
	if u.containerUid != 0 {
		capsToSet = intersect(capsToSet, r.nonRootMaxCaps)
	}
	var caps *specs.LinuxCapabilities
	// TODO centralize knowledge of garden -> runc capability schema translation
	if len(capsToSet) > 0 {
		caps = &specs.LinuxCapabilities{
			Bounding:    capsToSet,
			Inheritable: capsToSet,
			Permitted:   capsToSet,
		}
	}

	consoleBox := specs.Box{
		Width:  80,
		Height: 24,
	}
	if spec.TTY != nil && spec.TTY.WindowSize != nil {
		consoleBox.Width = uint(spec.TTY.WindowSize.Columns)
		consoleBox.Height = uint(spec.TTY.WindowSize.Rows)
	}

	return &PreparedSpec{
		HostUID: u.hostUid,
		HostGID: u.hostGid,
		Process: specs.Process{
			Args:        append([]string{spec.Path}, spec.Args...),
			ConsoleSize: consoleBox,
			Env:         r.envDeterminer.EnvFor(u.containerUid, bndl, spec),
			User: specs.User{
				UID:            uint32(u.containerUid),
				GID:            uint32(u.containerGid),
				AdditionalGids: []uint32{},
			},
			Cwd:             cwd,
			Capabilities:    caps,
			Rlimits:         toRlimits(spec.Limits),
			Terminal:        spec.TTY != nil,
			ApparmorProfile: bndl.Process().ApparmorProfile,
		},
	}, nil
}

type usr struct {
	hostUid, hostGid           int
	containerUid, containerGid int
	home                       string
}

func (r *execPreparer) lookupUser(bndl goci.Bndl, rootfsPath, username string) (*usr, error) {
	u, err := r.users.Lookup(rootfsPath, username)
	if err != nil {
		return nil, err
	}

	uid, gid := u.Uid, u.Gid
	if len(bndl.Spec.Linux.UIDMappings) > 0 {
		uid = idmapper.MappingList(bndl.Spec.Linux.UIDMappings).Map(uid)
		gid = idmapper.MappingList(bndl.Spec.Linux.GIDMappings).Map(gid)
	}

	return &usr{
		hostUid:      uid,
		hostGid:      gid,
		containerUid: u.Uid,
		containerGid: u.Gid,
		home:         u.Home,
	}, nil
}

func (r *execPreparer) ensureDirExists(rootFSPathFile, dir string, uid, gid int) error {
	if r.runningAsRoot() {
		// the MkdirAs throws a permission error when running in rootless mode...
		if err := r.mkdirer.MkdirAs(rootFSPathFile, uid, gid, 0755, false, dir); err != nil {
			return fmt.Errorf("create working directory: %s", err)
		}
	}

	return nil
}

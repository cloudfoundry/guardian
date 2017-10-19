package runrunc

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

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

	rootfsPath, ctrAlreadyCreated, err := getRootfsPath(log, bundlePath)
	if err != nil {
		return nil, err
	}

	user, err := r.lookupUser(bndl, rootfsPath, spec.User)
	if err != nil {
		log.Error("lookup-user-failed", err)
		return nil, err
	}

	cwd, err := r.getWorkingDir(log, ctrAlreadyCreated, spec, user, rootfsPath)
	if err != nil {
		log.Error("ensure-dir-failed", err)
		return nil, err
	}

	return &PreparedSpec{
		ContainerRootHostUID: containerRootHostID(bndl.Spec.Linux.UIDMappings),
		ContainerRootHostGID: containerRootHostID(bndl.Spec.Linux.GIDMappings),
		Process: specs.Process{
			Args:        append([]string{spec.Path}, spec.Args...),
			ConsoleSize: console(spec),
			Env:         r.envDeterminer.EnvFor(user.containerUID, bndl, spec),
			User: specs.User{
				UID:            uint32(user.containerUID),
				GID:            uint32(user.containerGID),
				AdditionalGids: []uint32{},
				Username:       spec.User,
			},
			Cwd:             cwd,
			Capabilities:    r.capabilities(bndl, user.containerUID),
			Rlimits:         toRlimits(spec.Limits),
			Terminal:        spec.TTY != nil,
			ApparmorProfile: bndl.Process().ApparmorProfile,
		},
	}, nil
}

func getRootfsPath(log lager.Logger, bundlePath string) (string, bool, error) {
	// We only look up the user in rootfspath/etc/passwd when the rootfs is
	// already mounted, as is the case for exec-ed processes, but not for peas.
	// For peas, spec.User must take the form uid:gid.
	pidBytes, err := ioutil.ReadFile(filepath.Join(bundlePath, "pidfile"))
	if err != nil {
		if os.IsNotExist(err) {
			log.Info("pidfile-not-present")
			return "", false, nil
		}
		log.Error("read-pidfile-failed", err)
		return "", false, err
	}

	return filepath.Join("/proc", string(pidBytes), "root"), true, nil
}

func (r *execPreparer) getWorkingDir(log lager.Logger, ctrAlreadyCreated bool, spec garden.ProcessSpec, user *usr, rootfsPath string) (string, error) {
	cwd := spec.Dir
	if cwd == "" {
		if ctrAlreadyCreated {
			cwd = user.home
		} else {
			cwd = "/"
		}
	}

	if err := r.ensureDirExists(ctrAlreadyCreated, rootfsPath, cwd, user.hostUID, user.hostGID); err != nil {
		return "", err
	}

	return cwd, nil
}

func (r *execPreparer) capabilities(bndl goci.Bndl, containerUID int) *specs.LinuxCapabilities {
	capsToSet := bndl.Capabilities()
	if containerUID != 0 {
		capsToSet = intersect(capsToSet, r.nonRootMaxCaps)
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

func console(spec garden.ProcessSpec) *specs.Box {
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

type usr struct {
	hostUID, hostGID           int
	containerUID, containerGID int
	home                       string
}

func (r *execPreparer) lookupUser(bndl goci.Bndl, rootfsPath, username string) (*usr, error) {
	if rootfsPath == "" && isUsername(username) {
		return nil, errors.New("processes that use an `Image` field must not use usernames: they may use `User` strings of the form uid:gid, defaulting to 0:0")
	}

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
		hostUID:      uid,
		hostGID:      gid,
		containerUID: u.Uid,
		containerGID: u.Gid,
		home:         u.Home,
	}, nil
}

func (r *execPreparer) ensureDirExists(ctrAlreadyCreated bool, rootFSPathFile, dir string, uid, gid int) error {
	// the MkdirAs throws a permission error when running in rootless mode...
	if r.runningAsRoot() && ctrAlreadyCreated {
		if err := r.mkdirer.MkdirAs(rootFSPathFile, uid, gid, 0755, false, dir); err != nil {
			return fmt.Errorf("create working directory: %s", err)
		}
	}

	return nil
}

func isUsername(username string) bool {
	return username != "" && !strings.Contains(username, ":")
}

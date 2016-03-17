package runrunc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden-shed/rootfs_provider"
	"github.com/cloudfoundry-incubator/goci"
	"github.com/opencontainers/specs"
	"github.com/pivotal-golang/lager"
)

type ExecPreparer struct {
	bundleLoader BundleLoader
	users        UserLookupper
	mkdirer      Mkdirer
}

func NewExecPreparer(bundleLoader BundleLoader, userlookup UserLookupper, mkdirer Mkdirer) *ExecPreparer {
	return &ExecPreparer{
		bundleLoader: bundleLoader,
		users:        userlookup,
		mkdirer:      mkdirer,
	}
}

func (r *ExecPreparer) Prepare(log lager.Logger, id, bundlePath, pidFilePath string, spec garden.ProcessSpec, runc RuncBinary) (*exec.Cmd, error) {
	log = log.Session("prepare")

	log.Info("start")
	defer log.Info("finished")

	if err := os.MkdirAll(path.Dir(pidFilePath), 0755); err != nil {
		log.Error("mk-process-dir-failed", err)
		return nil, err
	}

	bndl, err := r.bundleLoader.Load(bundlePath)
	if err != nil {
		log.Error("load-bundle-failed", err)
		return nil, err
	}

	rootFsPath := bndl.RootFS()
	u, err := r.lookupUser(bndl, rootFsPath, spec.User)
	if err != nil {
		log.Error("lookup-user-failed", err)
		return nil, err
	}

	cwd := u.home
	if spec.Dir != "" {
		cwd = spec.Dir
	}

	if err := r.ensureDirExists(rootFsPath, cwd, u.hostUid, u.hostGid); err != nil {
		log.Error("ensure-dir-failed", err)
		return nil, err
	}

	processJSON, err := writeProcessJSON(log, specs.Process{
		Args: append([]string{spec.Path}, spec.Args...),
		Env:  envFor(u.containerUid, bndl, spec),
		User: specs.User{
			UID: uint32(u.containerUid),
			GID: uint32(u.containerGid),
		},
		Cwd:          cwd,
		Capabilities: bndl.Capabilities(),
		Rlimits:      toRlimits(spec.Limits),
	})

	if err != nil {
		log.Error("encode-process-json-failed", err)
		return nil, err
	}

	return runc.ExecCommand(id, processJSON, pidFilePath), nil
}

type usr struct {
	hostUid, hostGid           int
	containerUid, containerGid int
	home                       string
}

func (r *ExecPreparer) lookupUser(bndl *goci.Bndl, rootfsPath, username string) (*usr, error) {
	u, err := r.users.Lookup(rootfsPath, username)
	if err != nil {
		return nil, err
	}

	uid, gid := u.Uid, u.Gid
	if len(bndl.Spec.Linux.UIDMappings) > 0 {
		uid = rootfs_provider.MappingList(bndl.Spec.Linux.UIDMappings).Map(uid)
		gid = rootfs_provider.MappingList(bndl.Spec.Linux.GIDMappings).Map(gid)
	}

	return &usr{
		hostUid:      uid,
		hostGid:      gid,
		containerUid: u.Uid,
		containerGid: u.Gid,
		home:         u.Home,
	}, nil
}

func (r *ExecPreparer) ensureDirExists(rootfsPath, dir string, uid, gid int) error {
	if err := r.mkdirer.MkdirAs(rootfsPath, uid, gid, 0755, false, dir); err != nil {
		return fmt.Errorf("create working directory: %s", err)
	}

	return nil
}

func writeProcessJSON(log lager.Logger, spec specs.Process) (string, error) {
	tmpFile, err := ioutil.TempFile("", "guardianprocess")
	if err != nil {
		log.Error("tempfile-failed", err)
		return "", err
	}

	if err := json.NewEncoder(tmpFile).Encode(spec); err != nil {
		log.Error("encode-failed", err)
		return "", fmt.Errorf("writeProcessJSON: %s", err)
	}

	return tmpFile.Name(), nil
}

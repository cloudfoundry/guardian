package runrunc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden-shed/rootfs_provider"
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

func (r *ExecPreparer) Prepare(log lager.Logger, id, bundlePath string, spec garden.ProcessSpec, runc RuncBinary) (*exec.Cmd, error) {
	bndl, err := r.bundleLoader.Load(bundlePath)
	if err != nil {
		return nil, err
	}

	tmpFile, err := ioutil.TempFile("", "guardianprocess")
	if err != nil {
		log.Error("tempfile-failed", err)
		return nil, err
	}

	rootFsPath := bndl.RootFS()
	if rootFsPath == "" {
		return nil, fmt.Errorf("empty rootfs path")
	}

	user, err := r.users.Lookup(rootFsPath, spec.User)
	if err != nil {
		return nil, err
	}

	cwd := user.Home
	if spec.Dir != "" {
		cwd = spec.Dir
	}

	uid, gid := user.Uid, user.Gid
	if len(bndl.Spec.Linux.UIDMappings) > 0 {
		uid = rootfs_provider.MappingList(bndl.Spec.Linux.UIDMappings).Map(uid)
		gid = rootfs_provider.MappingList(bndl.Spec.Linux.GIDMappings).Map(gid)
	}

	if err := r.mkdirer.MkdirAs(filepath.Join(rootFsPath, cwd), 0755, uid, gid); err != nil {
		return nil, fmt.Errorf("create working directory: %s", err)
	}

	defaultPath := DefaultPath
	if user.Uid == 0 {
		defaultPath = DefaultRootPath
	}

	env := envWithDefaultPath(append(
		bndl.Spec.Spec.Process.Env, spec.Env...,
	), defaultPath)

	if err := json.NewEncoder(tmpFile).Encode(specs.Process{
		Args: append([]string{spec.Path}, spec.Args...),
		Env:  env,
		User: specs.User{
			UID: uint32(user.Uid),
			GID: uint32(user.Gid),
		},
		Cwd: cwd,
	}); err != nil {
		log.Error("encode-failed", err)
		return nil, fmt.Errorf("writeProcessJSON: %s", err)
	}

	return runc.ExecCommand(id, tmpFile.Name()), nil
}

func envWithDefaultPath(env []string, defaultPath string) []string {
	for _, envVar := range env {
		if strings.Contains(envVar, "PATH=") {
			return env
		}
	}

	return append(env, defaultPath)
}

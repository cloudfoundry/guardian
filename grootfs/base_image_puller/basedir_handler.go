package base_image_puller

import (
	"os"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/sandbox"
	"code.cloudfoundry.org/lager/v3"
	"github.com/containers/storage/pkg/reexec"
	errorspkg "github.com/pkg/errors"
)

func init() {
	sandbox.Register("ensure-base", func(logger lager.Logger, extraFiles []*os.File, args ...string) error {
		return ensureBaseDirectoryExists(logger, args[0], args[1], args[2])
	})

	if reexec.Init() {
		// prevents infinite reexec loop
		// Details: https://medium.com/@teddyking/namespaces-in-go-reexec-3d1295b91af8
		os.Exit(0)
	}
}

type BasedirHandler struct {
	shouldCloneUserNsOnHandle bool
	reexecer                  groot.SandboxReexecer
}

func NewBasedirHandler(reexecer groot.SandboxReexecer, shouldCloneUserNsOnHandle bool) BaseDirHandler {
	return &BasedirHandler{
		shouldCloneUserNsOnHandle: shouldCloneUserNsOnHandle,
		reexecer:                  reexecer,
	}
}

func (h *BasedirHandler) Handle(logger lager.Logger, spec UnpackSpec, parentPath string) error {
	rootDir := filepath.Dir(parentPath)
	targetRelPath, err := filepath.Rel(rootDir, spec.TargetPath)
	if err != nil {
		return err
	}
	parentRelPath, err := filepath.Rel(rootDir, parentPath)
	if err != nil {
		return err
	}

	if _, err := h.reexecer.Reexec("ensure-base", groot.ReexecSpec{
		ChrootDir:   rootDir,
		Args:        []string{spec.BaseDirectory, targetRelPath, parentRelPath},
		CloneUserns: h.shouldCloneUserNsOnHandle,
	}); err != nil {
		return err
	}
	return nil
}

func ensureBaseDirectoryExists(logger lager.Logger, baseDir, childPath, parentPath string) error {
	if baseDir == string(filepath.Separator) {
		return nil
	}

	if err := ensureBaseDirectoryExists(logger, filepath.Dir(baseDir), childPath, parentPath); err != nil {
		return err
	}

	fullChildBaseDir := filepath.Join(childPath, baseDir)
	_, err := os.Stat(fullChildBaseDir)
	if err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return errorspkg.Wrap(err, "failed to stat base directory")
	}

	stat, err := os.Stat(filepath.Join(parentPath, baseDir))
	if err != nil {
		return errorspkg.Wrap(err, "base directory not found in parent layer")
	}

	if err := os.Mkdir(fullChildBaseDir, stat.Mode()); err != nil {
		return errorspkg.Wrap(err, "could not create base directory in child layer")
	}

	// we need to explicitly apply perms because mkdir is subject to umask
	if err := os.Chmod(fullChildBaseDir, stat.Mode()); err != nil {
		return errorspkg.Wrapf(err, "chmoding directory `%s`", fullChildBaseDir)
	}

	stat_t := stat.Sys().(*syscall.Stat_t)
	if err := os.Chown(fullChildBaseDir, int(stat_t.Uid), int(stat_t.Gid)); err != nil {
		return errorspkg.Wrap(err, "could not chown base directory")
	}

	return nil
}

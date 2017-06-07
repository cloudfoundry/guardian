package guardiancmd

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/commandrunner/windows_command_runner"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/execrunner"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type NoopStarter struct{}
type NoopChowner struct{}

func (s *NoopStarter) Start() error {
	return nil
}

func (c *NoopChowner) Chown(path string, uid, gid int) error {
	return nil
}

func commandRunner() commandrunner.CommandRunner {
	return windows_command_runner.New(false)
}

func wireDepot(depotPath string, bundleSaver depot.BundleSaver) *depot.DirectoryDepot {
	return depot.New(depotPath, bundleSaver, &NoopChowner{})
}

func (cmd *ServerCommand) wireVolumeCreator(logger lager.Logger, graphRoot string, insecureRegistries, persistentImages []string) gardener.VolumeCreator {
	if cmd.Image.Plugin.Path() != "" || cmd.Image.PrivilegedPlugin.Path() != "" {
		return cmd.wireImagePlugin()
	}

	return gardener.NoopVolumeCreator{}
}

func (cmd *ServerCommand) wireExecRunner(dadooPath, runcPath, runcRoot string, processIDGen runrunc.UidGenerator, commandRunner commandrunner.CommandRunner, shouldCleanup bool) *execrunner.DirectExecRunner {
	return &execrunner.DirectExecRunner{
		RuntimePath:   runcPath,
		CommandRunner: windows_command_runner.New(false),
		ProcessIDGen:  processIDGen,
		FileWriter:    &fileWriter{},
	}
}

func (cmd *ServerCommand) wireCgroupsStarter(logger lager.Logger) gardener.Starter {
	return &NoopStarter{}
}

type fileWriter struct{}

func (fileWriter) WriteFile(path string, contents []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0600); err != nil {
		return err
	}
	return ioutil.WriteFile(path, contents, mode)
}

func (cmd *ServerCommand) wireExecPreparer() runrunc.ExecPreparer {
	return &runrunc.WindowsExecPreparer{}
}

func defaultBindMounts(binInitPath string) []specs.Mount {
	return []specs.Mount{}
}

func privilegedMounts() []specs.Mount {
	return []specs.Mount{}
}

func unprivilegedMounts() []specs.Mount {
	return []specs.Mount{}
}

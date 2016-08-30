package volplugin

import (
	"bytes"
	"fmt"
	"os/exec"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden-shed/rootfs_provider"
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/gunk/command_runner"
)

func NewGrootfsVC(binPath, storePath string, commandRunner command_runner.CommandRunner) *GrootfsVC {
	return &GrootfsVC{
		binPath:       binPath,
		storePath:     storePath,
		commandRunner: commandRunner,
	}
}

type GrootfsVC struct {
	binPath       string
	storePath     string
	commandRunner command_runner.CommandRunner
}

func (vc *GrootfsVC) Create(log lager.Logger, handle string, spec rootfs_provider.Spec) (string, []string, error) {
	log = log.Session("grootfs-create")
	log.Debug("start")
	defer log.Debug("end")

	cmd := exec.Command(
		vc.binPath,
		"--store", vc.storePath,
		"create",
		spec.RootFS.String(),
		handle,
	)

	errBuffer := bytes.NewBuffer([]byte{})
	cmd.Stderr = errBuffer
	outBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = outBuffer

	if err := vc.commandRunner.Run(cmd); err != nil {
		return "", nil, fmt.Errorf("running grootfs create: %s - %s", err, errBuffer.String())
	}

	return outBuffer.String(), []string{}, nil
}
func (vc *GrootfsVC) Destroy(log lager.Logger, handle string) error {
	log = log.Session("grootfs-destroy")
	log.Debug("start")
	defer log.Debug("end")

	cmd := exec.Command(
		vc.binPath,
		"--store", vc.storePath,
		"delete",
		handle,
	)

	errBuffer := bytes.NewBuffer([]byte{})
	cmd.Stderr = errBuffer

	if err := vc.commandRunner.Run(cmd); err != nil {
		return fmt.Errorf("running grootfs delete: %s - %s", err, errBuffer.String())
	}

	return nil
}
func (vc *GrootfsVC) Metrics(log lager.Logger, handle string) (garden.ContainerDiskStat, error) {
	log = log.Session("grootfs-metrics")
	log.Debug("start")
	defer log.Debug("end")

	return garden.ContainerDiskStat{}, nil
}
func (vc *GrootfsVC) GC(log lager.Logger) error {
	log = log.Session("grootfs-gc")
	log.Debug("start")
	defer log.Debug("end")

	return nil
}

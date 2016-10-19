package imageplugin

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden-shed/rootfs_provider"
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/gunk/command_runner"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func New(binPath string, commandRunner command_runner.CommandRunner, mappings []specs.IDMapping) *ExternalImageManager {
	return &ExternalImageManager{
		binPath:       binPath,
		commandRunner: commandRunner,
		mappings:      mappings,
	}
}

type ExternalImageManager struct {
	binPath       string
	commandRunner command_runner.CommandRunner
	mappings      []specs.IDMapping
}

func (p *ExternalImageManager) Create(log lager.Logger, handle string, spec rootfs_provider.Spec) (string, []string, error) {
	log = log.Session("image-plugin-create")
	log.Debug("start")
	defer log.Debug("end")

	args := []string{"create"}
	if spec.QuotaSize != 0 {
		args = append(args, "--disk-limit-size-bytes", strconv.FormatInt(spec.QuotaSize, 10))
	}

	if spec.Namespaced {
		for _, mapping := range p.mappings {
			args = append(args, "--uid-mapping", stringifyMapping(mapping))
			args = append(args, "--gid-mapping", stringifyMapping(mapping))
		}
	}

	args = append(args, spec.RootFS.String(), handle)
	cmd := exec.Command(p.binPath, args...)

	errBuffer := bytes.NewBuffer([]byte{})
	cmd.Stderr = errBuffer
	outBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = outBuffer

	if err := p.commandRunner.Run(cmd); err != nil {
		logData := lager.Data{"action": "create", "stderr": errBuffer.String(), "stdout": outBuffer.String()}
		log.Error("external-image-manager-result", err, logData)
		return "", nil, fmt.Errorf("external image manager create failed: %s", err)
	}

	trimmedOut := strings.TrimSpace(outBuffer.String())
	rootFS := fmt.Sprintf("%s/rootfs", trimmedOut)
	return rootFS, []string{}, nil
}

func stringifyMapping(mapping specs.IDMapping) string {
	return fmt.Sprintf("%d:%d:%d", mapping.ContainerID, mapping.HostID, mapping.Size)
}

func (p *ExternalImageManager) Destroy(log lager.Logger, handle string) error {
	log = log.Session("image-plugin-destroy")
	log.Debug("start")
	defer log.Debug("end")

	cmd := exec.Command(
		p.binPath,
		"delete",
		handle,
	)

	errBuffer := bytes.NewBuffer([]byte{})
	cmd.Stderr = errBuffer

	if err := p.commandRunner.Run(cmd); err != nil {
		logData := lager.Data{"action": "delete", "stderr": errBuffer.String()}
		log.Error("external-image-manager-result", err, logData)
		return fmt.Errorf("external image manager destroy failed: %s", err)
	}

	return nil
}

func (p *ExternalImageManager) Metrics(log lager.Logger, handle string) (garden.ContainerDiskStat, error) {
	log = log.Session("image-plugin-metrics")
	log.Debug("start")
	defer log.Debug("end")

	return garden.ContainerDiskStat{}, nil
}

func (p *ExternalImageManager) GC(log lager.Logger) error {
	log = log.Session("image-plugin-gc")
	log.Debug("start")
	defer log.Debug("end")

	return nil
}

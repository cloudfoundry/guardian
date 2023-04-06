package imageplugin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/lager/v3"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	errorwrapper "github.com/pkg/errors"
)

const PreloadedPlusLayerScheme = "preloaded+layer"

//go:generate counterfeiter . CommandCreator
type CommandCreator interface {
	CreateCommand(log lager.Logger, handle string, spec gardener.RootfsSpec) (*exec.Cmd, error)
	DestroyCommand(log lager.Logger, handle string) *exec.Cmd
	MetricsCommand(log lager.Logger, handle string) *exec.Cmd
	CapacityCommand(log lager.Logger) *exec.Cmd
}

//go:generate counterfeiter . ImageSpecCreator
type ImageSpecCreator interface {
	CreateImageSpec(rootFS *url.URL, handle string) (*url.URL, error)
}

type ImagePlugin struct {
	UnprivilegedCommandCreator CommandCreator
	PrivilegedCommandCreator   CommandCreator
	ImageSpecCreator           ImageSpecCreator
	CommandRunner              commandrunner.CommandRunner
	DefaultRootfs              string
}

func (p *ImagePlugin) Create(log lager.Logger, handle string, spec gardener.RootfsSpec) (specs.Spec, error) {
	errs := func(err error, action string) (specs.Spec, error) {
		return specs.Spec{}, errorwrapper.Wrap(err, action)
	}

	log = log.Session("image-plugin-create", lager.Data{"handle": handle, "spec": spec})
	log.Debug("start")
	defer log.Debug("end")

	if spec.RootFS.String() == "" {
		var err error
		spec.RootFS, err = url.Parse(p.DefaultRootfs)

		if err != nil {
			log.Error("parsing-default-rootfs-failed", err)
			return errs(err, "parsing default rootfs")
		}
	}

	if strings.HasPrefix(spec.RootFS.String(), fmt.Sprintf("%s:", PreloadedPlusLayerScheme)) {
		var err error
		spec.RootFS, err = p.ImageSpecCreator.CreateImageSpec(spec.RootFS, handle)
		if err != nil {
			log.Error("creating-image-spec", err)
			return errs(err, "creating image spec")
		}
	}

	var (
		createCmd *exec.Cmd
		err       error
	)
	if spec.Namespaced {
		createCmd, err = p.UnprivilegedCommandCreator.CreateCommand(log, handle, spec)
	} else {
		createCmd, err = p.PrivilegedCommandCreator.CreateCommand(log, handle, spec)
	}
	if err != nil {
		return errs(err, "creating create command")
	}

	stdoutBuffer := bytes.NewBuffer([]byte{})
	createCmd.Stdout = stdoutBuffer
	createCmd.Stderr = NewRelogger(log)

	if err := p.CommandRunner.Run(createCmd); err != nil {
		logData := lager.Data{"action": "create", "stdout": stdoutBuffer.String()}
		log.Error("image-plugin-result", err, logData)
		return errs(err, fmt.Sprintf("running image plugin create: %s", stdoutBuffer.String()))
	}

	var runtimeSpec specs.Spec
	if err := json.Unmarshal(stdoutBuffer.Bytes(), &runtimeSpec); err != nil {
		logData := lager.Data{"action": "create", "stdout": stdoutBuffer.String()}
		log.Error("image-plugin-parsing", err, logData)
		return errs(err, fmt.Sprintf("parsing image plugin create: %s", stdoutBuffer.String()))
	}

	// Allow spec.Process.Env to be accessed without nil checks everywhere
	if runtimeSpec.Process == nil {
		runtimeSpec.Process = &specs.Process{}
	}

	// Allow spec.Root.Path to be accessed without nil checks everywhere
	if runtimeSpec.Root == nil {
		runtimeSpec.Root = &specs.Root{}
	}

	return runtimeSpec, nil
}

func (p *ImagePlugin) Destroy(log lager.Logger, handle string) error {
	log = log.Session("image-plugin-destroy", lager.Data{"handle": handle})
	log.Debug("start")
	defer log.Debug("end")

	var destroyCmds []*exec.Cmd
	destroyCmds = append(destroyCmds, p.UnprivilegedCommandCreator.DestroyCommand(log, handle))
	destroyCmds = append(destroyCmds, p.PrivilegedCommandCreator.DestroyCommand(log, handle))

	for _, destroyCmd := range destroyCmds {
		if destroyCmd == nil {
			continue
		}
		stdoutBuffer := bytes.NewBuffer([]byte{})
		destroyCmd.Stdout = stdoutBuffer
		destroyCmd.Stderr = NewRelogger(log)

		if err := p.CommandRunner.Run(destroyCmd); err != nil {
			logData := lager.Data{"action": "destroy", "stdout": stdoutBuffer.String()}
			log.Error("image-plugin-result", err, logData)
			return errorwrapper.Wrapf(err, "running image plugin destroy: %s", stdoutBuffer.String())
		}
	}

	return nil
}

func (p *ImagePlugin) Metrics(log lager.Logger, handle string, namespaced bool) (garden.ContainerDiskStat, error) {
	log = log.Session("image-plugin-metrics", lager.Data{"handle": handle, "namespaced": namespaced})
	log.Debug("start")
	defer log.Debug("end")

	var metricsCmd *exec.Cmd
	if namespaced {
		metricsCmd = p.UnprivilegedCommandCreator.MetricsCommand(log, handle)
	} else {
		metricsCmd = p.PrivilegedCommandCreator.MetricsCommand(log, handle)
	}

	if metricsCmd == nil {
		return garden.ContainerDiskStat{}, errors.New("requested image plugin not available")
	}

	stdoutBuffer := bytes.NewBuffer([]byte{})
	metricsCmd.Stdout = stdoutBuffer
	metricsCmd.Stderr = NewRelogger(log)

	if err := p.CommandRunner.Run(metricsCmd); err != nil {
		logData := lager.Data{"action": "metrics", "stdout": stdoutBuffer.String()}
		log.Error("image-plugin-result", err, logData)
		return garden.ContainerDiskStat{}, errorwrapper.Wrapf(err, "running image plugin metrics: %s", stdoutBuffer.String())
	}

	var diskStat map[string]map[string]uint64
	var consumableBuffer = bytes.NewBuffer(stdoutBuffer.Bytes())
	if err := json.NewDecoder(consumableBuffer).Decode(&diskStat); err != nil {
		return garden.ContainerDiskStat{}, errorwrapper.Wrapf(err, "parsing stats: %s", stdoutBuffer.String())
	}

	return garden.ContainerDiskStat{
		TotalBytesUsed:     diskStat["disk_usage"]["total_bytes_used"],
		ExclusiveBytesUsed: diskStat["disk_usage"]["exclusive_bytes_used"],
	}, nil
}

func (p *ImagePlugin) GC(log lager.Logger) error {
	return nil
}

func (p *ImagePlugin) Capacity(log lager.Logger) (uint64, error) {
	log = log.Session("image-plugin-capacity")
	log.Debug("start")
	defer log.Debug("end")

	var capacityCmd *exec.Cmd
	capacityCmd = p.UnprivilegedCommandCreator.CapacityCommand(log)

	stdoutBuffer := bytes.NewBuffer([]byte{})
	capacityCmd.Stdout = stdoutBuffer
	capacityCmd.Stderr = NewRelogger(log)

	err := p.CommandRunner.Run(capacityCmd)
	if err != nil {
		logData := lager.Data{"action": "capacity", "stdout": stdoutBuffer.String()}
		log.Error("image-plugin-result", err, logData)
		return 0, errorwrapper.Wrapf(err, "running image plugin capacity: %s", stdoutBuffer.String())
	}

	var capacityObject map[string]uint64
	var consumableBuffer = bytes.NewBuffer(stdoutBuffer.Bytes())
	err = json.NewDecoder(consumableBuffer).Decode(&capacityObject)
	if err != nil {
		return 0, errorwrapper.Wrapf(err, "parsing capacity: %s", stdoutBuffer.String())
	}

	return capacityObject["capacity"], nil
}

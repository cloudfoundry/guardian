package gardener

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	uuid "github.com/nu7hatch/gouuid"
)

type container struct {
	logger lager.Logger

	handle          string
	containerizer   Containerizer
	volumizer       Volumizer
	networker       Networker
	propertyManager PropertyManager

	gardener *Gardener
}

func (c *container) Handle() string {
	return c.handle
}

func (c *container) Run(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	if isPea(spec) {
		peaContainer, err := c.gardener.CreatePeaContainer(c.handle, spec.ID, spec.Image, spec.OverrideContainerLimits, spec.BindMounts)
		if err != nil {
			return nil, err
		}

		// if shouldResolveUsername(spec.User) {
		// resolvedUID, resolvedGID, err := c.peaUsernameResolver.ResolveUser(log, handle, spec.Image, spec.User)
		// if err != nil {
		// 	return nil, err
		// }

		// spec.User = fmt.Sprintf("%d:%d", resolvedUID, resolvedGID)
		// }

		process, err := peaContainer.Run(peaProcessSpec(spec), io)
		if err != nil {
			destroyErr := c.gardener.Destroy(c.handle)
			c.logger.Error("failed-to-destroy-pea-contianer", destroyErr)
			return nil, err
		}

		return NewPeaProcess(c.logger, process, peaContainer.Handle(), c.gardener), nil
		// return c.peaCreator.CreatePea(log, spec, io, handle)
	}

	if spec.BindMounts != nil {
		err := fmt.Errorf("Running a process with bind mounts and no image provided is not allowed")
		c.logger.Error("invalid-spec", err)
		return nil, err
	}

	return c.containerizer.Run(c.logger, c.handle, spec, io)
}

func peaProcessSpec(spec garden.ProcessSpec) garden.ProcessSpec {
	spec.Image = garden.ImageRef{}
	spec.BindMounts = nil
	id, _ := uuid.NewV4()
	spec.ID = id.String()
	return spec
}

func isPea(spec garden.ProcessSpec) bool {
	return spec.Image != (garden.ImageRef{})
}

// func shouldResolveUsername(username string) bool {
// 	return username != "" && !strings.Contains(username, ":")
// }

func (c *container) Attach(processID string, io garden.ProcessIO) (garden.Process, error) {
	return c.containerizer.Attach(c.logger, c.handle, processID, io)
}

func (c *container) Stop(kill bool) error {
	return c.containerizer.Stop(c.logger, c.handle, kill)
}

func (c *container) Info() (garden.ContainerInfo, error) {
	log := c.logger.Session("info", lager.Data{"handle": c.handle})

	log.Debug("starting")
	defer log.Debug("finished")

	containerIP, _ := c.propertyManager.Get(c.handle, ContainerIPKey)
	hostIP, _ := c.propertyManager.Get(c.handle, BridgeIPKey)
	externalIP, _ := c.propertyManager.Get(c.handle, ExternalIPKey)

	actualContainerSpec, err := c.containerizer.Info(c.logger, c.handle)
	if err != nil {
		return garden.ContainerInfo{}, err
	}

	properties, err := c.propertyManager.All(c.handle)
	if err != nil {
		return garden.ContainerInfo{}, err
	}

	mappedPorts := []garden.PortMapping{}
	mappedPortsCfg, _ := c.propertyManager.Get(c.handle, MappedPortsKey)

	state := "active"
	if actualContainerSpec.Stopped {
		state = "stopped"
	}

	json.Unmarshal([]byte(mappedPortsCfg), &mappedPorts)
	return garden.ContainerInfo{
		State:         state,
		ContainerIP:   containerIP,
		HostIP:        hostIP,
		ExternalIP:    externalIP,
		ContainerPath: actualContainerSpec.BundlePath,
		Events:        actualContainerSpec.Events,
		Properties:    properties,
		MappedPorts:   mappedPorts,
	}, nil
}

func (c *container) StreamIn(spec garden.StreamInSpec) error {
	return c.containerizer.StreamIn(c.logger, c.handle, spec)
}

func (c *container) StreamOut(spec garden.StreamOutSpec) (io.ReadCloser, error) {
	return c.containerizer.StreamOut(c.logger, c.handle, spec)
}

func (c *container) LimitBandwidth(limits garden.BandwidthLimits) error {
	return nil
}

func (c *container) CurrentBandwidthLimits() (garden.BandwidthLimits, error) {
	return garden.BandwidthLimits{}, nil
}

func (c *container) LimitCPU(limits garden.CPULimits) error {
	return nil
}

func (c *container) CurrentCPULimits() (garden.CPULimits, error) {
	info, err := c.containerizer.Info(c.logger, c.handle)
	return info.Limits.CPU, err
}

func (c *container) LimitDisk(limits garden.DiskLimits) error {
	return nil
}

func (c *container) CurrentDiskLimits() (garden.DiskLimits, error) {
	return garden.DiskLimits{}, nil
}

func (c *container) LimitMemory(limits garden.MemoryLimits) error {
	return nil
}

func (c *container) CurrentMemoryLimits() (garden.MemoryLimits, error) {
	info, err := c.containerizer.Info(c.logger, c.handle)
	return info.Limits.Memory, err
}

func (c *container) NetIn(hostPort, containerPort uint32) (uint32, uint32, error) {
	return c.networker.NetIn(c.logger, c.handle, hostPort, containerPort)
}

func (c *container) NetOut(netOutRule garden.NetOutRule) error {
	return c.networker.NetOut(c.logger, c.handle, netOutRule)
}

func (c *container) BulkNetOut(netOutRules []garden.NetOutRule) error {
	return c.networker.BulkNetOut(c.logger, c.handle, netOutRules)
}

func (c *container) Metrics() (garden.Metrics, error) {
	actualContainerMetrics, err := c.containerizer.Metrics(c.logger, c.handle)
	if err != nil {
		return garden.Metrics{}, err
	}

	diskMetrics, err1 := c.volumizer.Metrics(c.logger, c.handle, true)
	if err1 != nil {
		diskMetrics, err = c.volumizer.Metrics(c.logger, c.handle, false)
		if err != nil {
			return garden.Metrics{}, fmt.Errorf("image plugin returned these errors:\nunprivileged: %s\nprivileged: %s", err1.Error(), err.Error())
		}
	}

	return garden.Metrics{
		CPUStat:        actualContainerMetrics.CPU,
		MemoryStat:     actualContainerMetrics.Memory,
		DiskStat:       diskMetrics,
		PidStat:        actualContainerMetrics.Pid,
		Age:            actualContainerMetrics.Age,
		CPUEntitlement: actualContainerMetrics.CPUEntitlement,
	}, nil
}

func (c *container) Properties() (garden.Properties, error) {
	return c.propertyManager.All(c.handle)
}

func (c *container) Property(name string) (string, error) {
	if prop, ok := c.propertyManager.Get(c.handle, name); ok {
		return prop, nil
	}

	return "", fmt.Errorf("property does not exist: %s", name)
}

func (c *container) SetProperty(name string, value string) error {
	c.propertyManager.Set(c.handle, name, value)
	return nil
}

func (c *container) RemoveProperty(name string) error {
	c.propertyManager.Remove(c.handle, name)
	return nil
}

func (c *container) SetGraceTime(t time.Duration) error {
	c.propertyManager.Set(c.handle, GraceTimeKey, fmt.Sprintf("%d", t))
	return nil
}

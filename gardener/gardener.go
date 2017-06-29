package gardener

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"time"

	specs "github.com/opencontainers/runtime-spec/specs-go"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden-shed/rootfs_spec"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . SysInfoProvider
//go:generate counterfeiter . Containerizer
//go:generate counterfeiter . Networker
//go:generate counterfeiter . VolumeCreator
//go:generate counterfeiter . UidGenerator
//go:generate counterfeiter . PropertyManager
//go:generate counterfeiter . Restorer
//go:generate counterfeiter . Starter
//go:generate counterfeiter . BulkStarter

const ContainerIPKey = "garden.network.container-ip"
const BridgeIPKey = "garden.network.host-ip"
const ExternalIPKey = "garden.network.external-ip"
const MappedPortsKey = "garden.network.mapped-ports"
const GraceTimeKey = "garden.grace-time"

const RawRootFSScheme = "raw"

const volumeCreatorSession = "volume-creator"

type SysInfoProvider interface {
	TotalMemory() (uint64, error)
	TotalDisk() (uint64, error)
}

type Containerizer interface {
	Create(log lager.Logger, spec DesiredContainerSpec) error
	Handles() ([]string, error)

	StreamIn(log lager.Logger, handle string, spec garden.StreamInSpec) error
	StreamOut(log lager.Logger, handle string, spec garden.StreamOutSpec) (io.ReadCloser, error)

	Run(log lager.Logger, handle string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error)
	Attach(log lager.Logger, handle string, processGUID string, io garden.ProcessIO) (garden.Process, error)
	Stop(log lager.Logger, handle string, kill bool) error
	Destroy(log lager.Logger, handle string) error
	RemoveBundle(log lager.Logger, handle string) error

	Info(log lager.Logger, handle string) (ActualContainerSpec, error)
	Metrics(log lager.Logger, handle string) (ActualContainerMetrics, error)
}

type Networker interface {
	Network(log lager.Logger, spec garden.ContainerSpec, pid int) error
	Capacity() uint64
	Destroy(log lager.Logger, handle string) error
	NetIn(log lager.Logger, handle string, hostPort, containerPort uint32) (uint32, uint32, error)
	BulkNetOut(log lager.Logger, handle string, rules []garden.NetOutRule) error
	NetOut(log lager.Logger, handle string, rule garden.NetOutRule) error
	Restore(log lager.Logger, handle string) error
}

type VolumeCreator interface {
	Create(log lager.Logger, handle string, spec rootfs_spec.Spec) (DesiredImageSpec, error)
	Destroy(log lager.Logger, handle string) error
	Metrics(log lager.Logger, handle string, privileged bool) (garden.ContainerDiskStat, error)
	GC(log lager.Logger) error
}

type DesiredImageSpec struct {
	RootFS string        `json:"rootfs,omitempty"`
	Mounts []specs.Mount `json:"mounts,omitempty"`
	Image  Image         `json:"image,omitempty"`
}

type Image struct {
	Config ImageConfig `json:"config,omitempty"`
}

type ImageConfig struct {
	Env []string `json:"Env,omitempty"`
}

type UidGenerator interface {
	Generate() string
}

type PropertyManager interface {
	All(handle string) (props garden.Properties, err error)
	Set(handle string, name string, value string)
	Remove(handle string, name string) error
	Get(handle string, name string) (string, bool)
	MatchesAll(handle string, props garden.Properties) bool
	DestroyKeySpace(string) error
}

type Starter interface {
	Start() error
}

type BulkStarter interface {
	StartAll() error
}

type Restorer interface {
	Restore(logger lager.Logger, handles []string) []string
}

type UidGeneratorFunc func() string

func (fn UidGeneratorFunc) Generate() string {
	return fn()
}

type DesiredContainerSpec struct {
	Handle string

	// Path to the Root Filesystem for the container
	RootFSPath string

	// Container hostname
	Hostname string

	// Bind mounts
	BindMounts []garden.BindMount

	// Mounts returned from the VolumeCreator
	DesiredImageSpecMounts []specs.Mount

	// Container is privileged
	Privileged bool

	Limits garden.Limits

	Env []string
}

type ActualContainerSpec struct {
	// The PID of the container's init process
	Pid int

	// The path to the container's bundle directory
	BundlePath string

	// The path to the container's rootfs
	RootFSPath string

	// Whether the container is stopped
	Stopped bool

	// Process IDs (not PIDs) of processes in the container
	ProcessIDs []string

	// Events (e.g. OOM) which have occured in the container
	Events []string

	// Applied limits
	Limits garden.Limits

	// Whether the container is privileged
	Privileged bool
}

type ActualContainerMetrics struct {
	CPU    garden.ContainerCPUStat
	Memory garden.ContainerMemoryStat
}

// Gardener orchestrates other components to implement the Garden API
type Gardener struct {
	// SysInfoProvider returns total memory and total disk
	SysInfoProvider SysInfoProvider

	// Containerizer runs and manages linux containers
	Containerizer Containerizer

	// UidGenerator generates unique ids for containers
	UidGenerator UidGenerator

	// BulkStarter runs any needed Starters that do start-up tasks (e.g. setting up cgroups)
	BulkStarter BulkStarter

	// Networker creates a network for containers
	Networker Networker

	// VolumeCreator creates volumes for containers
	VolumeCreator VolumeCreator

	Logger lager.Logger

	// PropertyManager creates map of container properties
	PropertyManager PropertyManager

	// MaxContainers limits the advertised container capacity
	MaxContainers uint64

	Restorer Restorer
}

// Create creates a container by combining the results of networker.Network,
// volumizer.Create and containzer.Create.
func (g *Gardener) Create(spec garden.ContainerSpec) (ctr garden.Container, err error) {
	log := g.Logger.Session("create", lager.Data{"handle": spec.Handle})
	log.Info("start")

	knownHandles, err := g.Containerizer.Handles()
	if err != nil {
		return nil, err
	}

	if err := g.checkDuplicateHandle(knownHandles, spec.Handle); err != nil {
		return nil, err
	}

	if err := g.checkMaxContainers(knownHandles); err != nil {
		return nil, err
	}

	if spec.Handle == "" {
		spec.Handle = g.UidGenerator.Generate()
	}

	defer func() {
		if err != nil {
			log := log.Session("create-failed-cleaningup", lager.Data{
				"cause": err.Error(),
			})

			log.Info("start")

			err := g.destroy(log, spec.Handle)
			if err != nil {
				log.Error("destroy-failed", err)
			}

			log.Info("cleanedup")
		} else {
			log.Info("created")
		}
	}()

	path := spec.Image.URI
	if path == "" {
		path = spec.RootFSPath
	} else if spec.RootFSPath != "" {
		return nil, errors.New("Cannot provide both Image.URI and RootFSPath")
	}

	rootFSURL, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	if err := g.VolumeCreator.GC(log.Session(volumeCreatorSession)); err != nil {
		log.Error("graph-cleanup-failed", err)
	}

	var desiredImageSpec DesiredImageSpec

	if rootFSURL.Scheme == RawRootFSScheme {
		desiredImageSpec.RootFS = rootFSURL.Path
	} else {
		var err error
		desiredImageSpec, err = g.VolumeCreator.Create(log.Session(volumeCreatorSession), spec.Handle, rootfs_spec.Spec{
			RootFS:     rootFSURL,
			Username:   spec.Image.Username,
			Password:   spec.Image.Password,
			QuotaSize:  int64(spec.Limits.Disk.ByteHard),
			QuotaScope: spec.Limits.Disk.Scope,
			Namespaced: !spec.Privileged,
		})
		if err != nil {
			return nil, err
		}
	}

	if err := g.Containerizer.Create(log, DesiredContainerSpec{
		Handle:                 spec.Handle,
		RootFSPath:             desiredImageSpec.RootFS,
		Hostname:               spec.Handle,
		Privileged:             spec.Privileged,
		BindMounts:             spec.BindMounts,
		DesiredImageSpecMounts: desiredImageSpec.Mounts,
		Limits:                 spec.Limits,
		Env:                    append(desiredImageSpec.Image.Config.Env, spec.Env...),
	}); err != nil {
		return nil, err
	}

	actualSpec, err := g.Containerizer.Info(log, spec.Handle)
	if err != nil {
		return nil, err
	}

	if err = g.Networker.Network(log, spec, actualSpec.Pid); err != nil {
		return nil, err
	}

	container, err := g.Lookup(spec.Handle)
	if err != nil {
		return nil, err
	}

	if spec.GraceTime != 0 {
		if err := container.SetGraceTime(spec.GraceTime); err != nil {
			return nil, err
		}
	}

	for name, value := range spec.Properties {
		if err := container.SetProperty(name, value); err != nil {
			return nil, err
		}
	}

	if err := container.SetProperty("garden.state", "created"); err != nil {
		return nil, err
	}

	return container, nil
}

func (g *Gardener) Lookup(handle string) (garden.Container, error) {
	return g.lookup(handle), nil
}

func (g *Gardener) lookup(handle string) garden.Container {
	return &container{
		logger:          g.Logger,
		handle:          handle,
		containerizer:   g.Containerizer,
		volumeCreator:   g.VolumeCreator,
		networker:       g.Networker,
		propertyManager: g.PropertyManager,
	}
}

func (g *Gardener) Destroy(handle string) error {
	log := g.Logger.Session("destroy", lager.Data{"handle": handle})

	log.Info("start")
	defer log.Info("finished")

	handles, err := g.Containerizer.Handles()
	if err != nil {
		return err
	}

	if !g.exists(handles, handle) {
		return garden.ContainerNotFoundError{Handle: handle}
	}

	return g.destroy(log, handle)
}

// destroy idempotently destroys any resources associated with the given handle
func (g *Gardener) destroy(log lager.Logger, handle string) error {
	if err := g.Containerizer.Destroy(log, handle); err != nil {
		return err
	}

	if err := g.Networker.Destroy(log, handle); err != nil {
		return err
	}

	if err := g.VolumeCreator.Destroy(log.Session(volumeCreatorSession), handle); err != nil {
		return err
	}

	if err := g.PropertyManager.DestroyKeySpace(handle); err != nil {
		return err
	}

	return g.Containerizer.RemoveBundle(log, handle)
}

func (g *Gardener) Stop() {}

func (g *Gardener) GraceTime(container garden.Container) time.Duration {
	property, ok := g.PropertyManager.Get(container.Handle(), GraceTimeKey)
	if !ok {
		return 0
	}

	var graceTime time.Duration
	_, err := fmt.Sscanf(property, "%d", &graceTime)
	if err != nil {
		return 0
	}

	return graceTime
}

func (g *Gardener) Ping() error { return nil }

func (g *Gardener) Capacity() (garden.Capacity, error) {
	mem, err := g.SysInfoProvider.TotalMemory()
	if err != nil {
		return garden.Capacity{}, err
	}

	disk, err := g.SysInfoProvider.TotalDisk()
	if err != nil {
		return garden.Capacity{}, err
	}

	cap := g.Networker.Capacity()
	if g.MaxContainers > 0 && g.MaxContainers < cap {
		cap = g.MaxContainers
	}

	return garden.Capacity{
		MemoryInBytes: mem,
		DiskInBytes:   disk,
		MaxContainers: cap,
	}, nil
}

func (g *Gardener) Containers(props garden.Properties) ([]garden.Container, error) {
	log := g.Logger.Session("list-containers")

	log.Info("starting")
	defer log.Info("finished")

	handles, err := g.Containerizer.Handles()
	if err != nil {
		log.Error("handles-failed", err)
		return []garden.Container{}, err
	}

	if props == nil {
		props = garden.Properties{}
	}
	props["garden.state"] = "created"

	var containers []garden.Container
	for _, handle := range handles {
		matched := g.PropertyManager.MatchesAll(handle, props)
		if matched {
			containers = append(containers, g.lookup(handle))
		}
	}

	return containers, nil
}

func (g *Gardener) BulkInfo(handles []string) (map[string]garden.ContainerInfoEntry, error) {
	result := make(map[string]garden.ContainerInfoEntry)
	for _, handle := range handles {
		container := g.lookup(handle)

		var infoErr *garden.Error = nil
		info, err := container.Info()
		if err != nil {
			infoErr = garden.NewError(err.Error())
		}
		result[handle] = garden.ContainerInfoEntry{
			Info: info,
			Err:  infoErr,
		}
	}

	return result, nil
}

func (g *Gardener) BulkMetrics(handles []string) (map[string]garden.ContainerMetricsEntry, error) {
	result := make(map[string]garden.ContainerMetricsEntry)
	for _, handle := range handles {
		var e *garden.Error
		m, err := g.lookup(handle).Metrics()
		if err != nil {
			e = garden.NewError(err.Error())
		}

		result[handle] = garden.ContainerMetricsEntry{
			Err:     e,
			Metrics: m,
		}
	}

	return result, nil
}

func (g *Gardener) checkDuplicateHandle(knownHandles []string, handle string) error {
	if g.exists(knownHandles, handle) {
		return fmt.Errorf("Handle '%s' already in use", handle)
	}

	return nil
}

func (g *Gardener) exists(handles []string, handle string) bool {
	for _, h := range handles {
		if h == handle {
			return true
		}
	}

	return false
}

func (g *Gardener) checkMaxContainers(handles []string) error {
	if g.MaxContainers == 0 {
		return nil
	}

	if len(handles) >= int(g.MaxContainers) {
		return errors.New("max containers reached")
	}

	return nil
}

func (g *Gardener) Start() error {
	log := g.Logger.Session("start")

	log.Info("starting")
	defer log.Info("completed")

	if err := g.BulkStarter.StartAll(); err != nil {
		return fmt.Errorf("bulk starter: %s", err)
	}

	handles, err := g.Containerizer.Handles()
	if err != nil {
		return err
	}

	for _, handle := range g.Restorer.Restore(log, handles) {
		destroyLog := log.Session("clean-up-container", lager.Data{"handle": handle})
		destroyLog.Info("start")

		if err := g.destroy(destroyLog, handle); err != nil {
			destroyLog.Error("failed", err)
			continue
		}

		destroyLog.Info("cleaned-up")
	}

	return nil
}

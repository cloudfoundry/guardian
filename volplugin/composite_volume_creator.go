package volplugin

import (
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden-shed/rootfs_provider"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/lager"
)

func NewCompositeVolumeCreator(grootfsVC, gardenShedVC gardener.VolumeCreator, pm gardener.PropertyManager) *CompositeVolumeCreator {
	return &CompositeVolumeCreator{
		grootfsVC:       grootfsVC,
		gardenShedVC:    gardenShedVC,
		propertyManager: pm,
	}
}

type CompositeVolumeCreator struct {
	propertyManager gardener.PropertyManager
	grootfsVC       gardener.VolumeCreator
	gardenShedVC    gardener.VolumeCreator
}

func (vc *CompositeVolumeCreator) Create(log lager.Logger, handle string, spec rootfs_provider.Spec) (string, []string, error) {
	log = log.Session("volume-plugin.creating", lager.Data{"handle": handle, "spec": spec})
	log.Debug("start")
	defer log.Debug("end")

	var volumeCreator gardener.VolumeCreator
	if strings.HasPrefix(spec.RootFS.Scheme, "grootfs+") {
		log.Debug("grootfs-volume-creator-selected")

		spec.RootFS.Scheme = strings.Replace(spec.RootFS.Scheme, "grootfs+", "", 1)
		vc.propertyManager.Set(handle, "volumePlugin", "grootfs")
		volumeCreator = vc.grootfsVC
	} else {
		log.Debug("garden-shed-volume-creator-selected")

		vc.propertyManager.Set(handle, "volumePlugin", "garden-shed")
		volumeCreator = vc.gardenShedVC
	}

	if err := volumeCreator.GC(log); err != nil {
		log.Error("graph-cleanup-failed", err)
	}

	return volumeCreator.Create(log, handle, spec)
}

func (vc *CompositeVolumeCreator) Destroy(log lager.Logger, handle string) error {
	log = log.Session("volume-plugin.destroying", lager.Data{"handle": handle})
	log.Debug("start")
	defer log.Debug("end")

	volumeCreator := vc.decideVolumeCreator(log, handle)
	return volumeCreator.Destroy(log, handle)
}

func (vc *CompositeVolumeCreator) Metrics(log lager.Logger, handle string) (garden.ContainerDiskStat, error) {
	log = log.Session("volume-plugin.metrics", lager.Data{"handle": handle})
	log.Debug("start")
	defer log.Debug("end")

	volumeCreator := vc.decideVolumeCreator(log, handle)
	return volumeCreator.Metrics(log, handle)
}

func (vc *CompositeVolumeCreator) GC(log lager.Logger) error {
	log = log.Session("volume-plugin.gc")
	log.Debug("start")
	defer log.Debug("end")

	return vc.gardenShedVC.GC(log)
}

func (vc *CompositeVolumeCreator) decideVolumeCreator(log lager.Logger, handle string) gardener.VolumeCreator {
	plugin, _ := vc.propertyManager.Get(handle, "volumePlugin")
	if plugin == "grootfs" {
		log.Debug("grootfs-volume-creator-selected")
		return vc.grootfsVC
	}

	log.Debug("garden-shed-volume-creator-selected")
	return vc.gardenShedVC
}

package kawasaki

import (
	"github.com/cloudfoundry-incubator/guardian/kawasaki/dns"
	"github.com/cloudfoundry-incubator/guardian/rundmc/goci"
)

type ResolvFactory struct{}

func (r *ResolvFactory) CreateDNSResolvConfigurer(bundlePath string, config NetworkConfig) DnsResolvConfigurer {
	bundleLoader := &goci.BndlLoader{}
	bndl, err := bundleLoader.Load(bundlePath)
	if err != nil {
		panic(err)
	}

	rootUid, rootGid := extractRootIds(bndl)

	configurer := &dns.ResolvConfigurer{
		HostsFileCompiler: &dns.HostsFileCompiler{
			Handle: config.ContainerHandle,
			IP:     config.ContainerIP,
		},
		ResolvFileCompiler: &dns.ResolvFileCompiler{
			HostResolvConfPath: "/etc/resolv.conf",
			HostIP:             config.BridgeIP,
			OverrideServers:    config.DNSServers,
		},
		FileWriter: &dns.RootfsWriter{
			RootfsPath: bndl.Spec.Root.Path,
			RootUid:    rootUid,
			RootGid:    rootGid,
		},
	}

	return configurer
}

func extractRootIds(bndl goci.Bndl) (int, int) {
	rootUid := 0
	for _, mapping := range bndl.Spec.Linux.UIDMappings {
		if mapping.ContainerID == 0 && mapping.Size >= 1 {
			rootUid = int(mapping.HostID)
			break
		}
	}

	rootGid := 0
	for _, mapping := range bndl.Spec.Linux.GIDMappings {
		if mapping.ContainerID == 0 && mapping.Size >= 1 {
			rootGid = int(mapping.HostID)
			break
		}
	}

	return rootUid, rootGid
}

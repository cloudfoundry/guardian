package kawasaki

import (
	"fmt"

	"code.cloudfoundry.org/guardian/kawasaki/dns"
)

type ResolvFactory struct {
	idMapReader dns.RootIdMapReader
}

func (r *ResolvFactory) CreateDNSResolvConfigurer(pid int, config NetworkConfig) (DnsResolvConfigurer, error) {
	rootUid, err := r.idMapReader.ReadRootId(fmt.Sprintf("/proc/%d/uid_map", pid))
	if err != nil {
		return nil, err
	}

	rootGid, err := r.idMapReader.ReadRootId(fmt.Sprintf("/proc/%d/gid_map", pid))
	if err != nil {
		return nil, err
	}

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
			RootfsPath: fmt.Sprintf("/proc/%d/root", pid),
			RootUid:    rootUid,
			RootGid:    rootGid,
		},
	}

	return configurer, nil
}

package netplugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os/exec"
	"strings"
	"time"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/kawasaki"
	"code.cloudfoundry.org/lager/v3"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

const NetworkPropertyPrefix = "network."

var AllowableProperties = map[string]struct{}{"log_config": {}}

type externalBinaryNetworker struct {
	commandRunner         commandrunner.CommandRunner
	configStore           kawasaki.ConfigStore
	externalIP            net.IP
	operatorNameservers   []net.IP
	additionalNameservers []net.IP
	resolvConfigurer      kawasaki.DnsResolvConfigurer
	path                  string
	extraArg              []string
	networkDepot          kawasaki.NetworkDepot
}

func New(
	commandRunner commandrunner.CommandRunner,
	configStore kawasaki.ConfigStore,
	externalIP net.IP,
	operatorNameServers []net.IP,
	additionalNameservers []net.IP,
	resolvConfigurer kawasaki.DnsResolvConfigurer,
	path string,
	extraArg []string,
	networkDepot kawasaki.NetworkDepot,
) ExternalNetworker {
	return &externalBinaryNetworker{
		commandRunner:         commandRunner,
		configStore:           configStore,
		externalIP:            externalIP,
		operatorNameservers:   operatorNameServers,
		additionalNameservers: additionalNameservers,
		resolvConfigurer:      resolvConfigurer,
		path:                  path,
		extraArg:              extraArg,
		networkDepot:          networkDepot,
	}
}

type ExternalNetworker interface {
	gardener.Networker
	gardener.Starter
}

func (p *externalBinaryNetworker) Start() error { return nil }

func networkProperties(containerProperties garden.Properties) garden.Properties {
	properties := garden.Properties{}

	for k, value := range containerProperties {
		if strings.HasPrefix(k, NetworkPropertyPrefix) {
			key := strings.TrimPrefix(k, NetworkPropertyPrefix)
			properties[key] = value
		} else if _, ok := AllowableProperties[k]; ok {
			properties[k] = value
		}
	}

	return properties
}

type UpInputs struct {
	Pid        int
	Properties map[string]string
	NetOut     []garden.NetOutRule `json:"netout_rules,omitempty"`
	NetIn      []garden.NetIn      `json:"netin,omitempty"`
}

type UpOutputs struct {
	Properties    map[string]string
	DNSServers    []string `json:"dns_servers,omitempty"`
	SearchDomains []string `json:"search_domains,omitempty"`
}

func (p *externalBinaryNetworker) SetupBindMounts(log lager.Logger, handle string, privileged bool, rootfsPath string) ([]garden.BindMount, error) {
	return p.networkDepot.SetupBindMounts(log, handle, privileged, rootfsPath)
}

func (p *externalBinaryNetworker) Network(log lager.Logger, containerSpec garden.ContainerSpec, pid int) error {
	p.configStore.Set(containerSpec.Handle, gardener.ExternalIPKey, p.externalIP.String())

	inputs := UpInputs{
		Pid:        pid,
		Properties: networkProperties(containerSpec.Properties),
		NetOut:     containerSpec.NetOut,
		NetIn:      containerSpec.NetIn,
	}

	outputs := UpOutputs{}
	err := p.exec(log, "up", containerSpec.Handle, inputs, &outputs)
	if err != nil {
		return err
	}

	// Ensure loopback interface is up in the container's network namespace.
	// External network plugins (e.g. silk CNI) typically only create a veth pair.
	// Runtimes like gVisor that build their own netstack by scraping the netns
	// need lo to be present for applications binding to 127.0.0.1.
	if err := setupLoopback(log, pid); err != nil {
		log.Error("setup-loopback", err)
		// Non-fatal: runc brings up lo on its own, only gVisor needs this.
	}

	for k, v := range outputs.Properties {
		p.configStore.Set(containerSpec.Handle, k, v)
	}

	var pluginNameservers []net.IP
	if outputs.DNSServers != nil {
		pluginNameservers = []net.IP{}
		for _, dnsServer := range outputs.DNSServers {
			pluginNameservers = append(pluginNameservers, net.ParseIP(dnsServer))
		}
	}

	containerIP, ok := p.configStore.Get(containerSpec.Handle, gardener.ContainerIPKey)

	// ipv6 address is "" if not returned by external networker
	containerIPv6, _ := p.configStore.Get(containerSpec.Handle, gardener.ContainerIPv6Key)
	if ok {
		log.Info("external-binary-write-dns-to-config", lager.Data{
			"dnsServers": pluginNameservers,
		})
		cfg := kawasaki.NetworkConfig{
			ContainerIP:           net.ParseIP(containerIP),
			ContainerIPv6:         net.ParseIP(containerIPv6),
			BridgeIP:              net.ParseIP(containerIP),
			ContainerHandle:       containerSpec.Handle,
			OperatorNameservers:   p.operatorNameservers,
			AdditionalNameservers: p.additionalNameservers,
			PluginNameservers:     pluginNameservers,
			PluginSearchDomains:   outputs.SearchDomains,
		}

		err = p.resolvConfigurer.Configure(log, cfg, pid)
		if err != nil {
			return err
		}
	}

	return nil
}

// setupLoopback enters the container's network namespace and brings up the
// loopback interface. This is needed for runtimes like gVisor that scrape the
// netns for interfaces rather than creating lo themselves.
func setupLoopback(log lager.Logger, pid int) error {
	containerNS, err := netns.GetFromPid(pid)
	if err != nil {
		return fmt.Errorf("getting netns for pid %d: %w", pid, err)
	}
	defer containerNS.Close()

	// Create a netlink handle in the container's network namespace.
	nlh, err := netlink.NewHandleAt(containerNS)
	if err != nil {
		return fmt.Errorf("creating netlink handle in container netns: %w", err)
	}
	defer nlh.Close()

	lo, err := nlh.LinkByName("lo")
	if err != nil {
		return fmt.Errorf("finding lo interface: %w", err)
	}

	if err := nlh.LinkSetUp(lo); err != nil {
		return fmt.Errorf("bringing up lo: %w", err)
	}

	log.Info("setup-loopback-success", lager.Data{"pid": pid})
	return nil
}

// warmARPCache ensures the container's network namespace has resolved ARP
// entries for all gateway IPs. gVisor's netstack copies ARP entries from the
// namespace at startup but only those in "live" states (PERMANENT, REACHABLE,
// STALE). On silk CNI with point-to-point links, gateway ARP entries may not
// exist until triggered. This function probes each gateway with a UDP packet
// to force ARP resolution, then promotes entries to PERMANENT so gVisor
// reliably picks them up.
func warmARPCache(log lager.Logger, pid int) error {
	containerNS, err := netns.GetFromPid(pid)
	if err != nil {
		return fmt.Errorf("getting netns for pid %d: %w", pid, err)
	}
	defer containerNS.Close()

	nlh, err := netlink.NewHandleAt(containerNS)
	if err != nil {
		return fmt.Errorf("creating netlink handle: %w", err)
	}
	defer nlh.Close()

	// Find all gateway IPs from routes in the container namespace.
	routes, err := nlh.RouteList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return fmt.Errorf("listing routes: %w", err)
	}

	gwSet := map[string]net.IP{}
	for _, r := range routes {
		if r.Gw != nil && !r.Gw.IsUnspecified() {
			gwSet[r.Gw.String()] = r.Gw
		}
	}

	if len(gwSet) == 0 {
		return nil
	}

	// Check which gateways already have ARP entries.
	links, _ := nlh.LinkList()
	var primaryLink netlink.Link
	for _, l := range links {
		if l.Attrs().Name != "lo" {
			primaryLink = l
			break
		}
	}

	if primaryLink == nil {
		return nil
	}

	neighs, _ := nlh.NeighList(primaryLink.Attrs().Index, netlink.FAMILY_ALL)
	hasEntry := func(ip net.IP) bool {
		for _, n := range neighs {
			if n.IP.Equal(ip) && len(n.HardwareAddr) > 0 {
				return true
			}
		}
		return false
	}

	// Probe unresolved gateways with a UDP packet to trigger ARP.
	probed := false
	for _, gw := range gwSet {
		if hasEntry(gw) {
			continue
		}
		network := "udp4"
		if gw.To4() == nil {
			network = "udp6"
		}
		conn, err := net.DialUDP(network, nil, &net.UDPAddr{IP: gw, Port: 9})
		if err != nil {
			log.Info("arp-warm-probe-failed", lager.Data{"gw": gw.String(), "error": err.Error()})
			continue
		}
		conn.Write([]byte{})
		conn.Close()
		probed = true
	}

	if probed {
		time.Sleep(75 * time.Millisecond) // wait for ARP reply
	}

	// Re-read neighbor table and promote entries to PERMANENT.
	neighs, err = nlh.NeighList(primaryLink.Attrs().Index, netlink.FAMILY_ALL)
	if err != nil {
		return fmt.Errorf("re-reading neighbor table: %w", err)
	}

	const liveStates = netlink.NUD_REACHABLE | netlink.NUD_STALE | netlink.NUD_DELAY | netlink.NUD_PROBE | netlink.NUD_PERMANENT
	promoted := 0
	for _, n := range neighs {
		if len(n.HardwareAddr) == 0 || n.State&liveStates == 0 {
			continue
		}
		if _, isGW := gwSet[n.IP.String()]; !isGW {
			continue
		}
		if n.State == netlink.NUD_PERMANENT {
			continue // already permanent
		}
		// Promote to PERMANENT so gVisor's scraper picks it up.
		err := nlh.NeighSet(&netlink.Neigh{
			LinkIndex:    primaryLink.Attrs().Index,
			IP:           n.IP,
			HardwareAddr: n.HardwareAddr,
			State:        netlink.NUD_PERMANENT,
		})
		if err != nil {
			log.Info("arp-promote-failed", lager.Data{"ip": n.IP.String(), "error": err.Error()})
		} else {
			promoted++
		}
	}

	log.Info("warm-arp-cache-done", lager.Data{"pid": pid, "gateways": len(gwSet), "promoted": promoted})
	return nil
}

func (p *externalBinaryNetworker) Destroy(log lager.Logger, handle string) error {
	err := p.exec(log, "down", handle, nil, nil)
	if err != nil {
		return err
	}

	return p.networkDepot.Destroy(log, handle)
}

func (p *externalBinaryNetworker) Restore(log lager.Logger, handle string) error {
	return nil
}

func (p *externalBinaryNetworker) Capacity() (m uint64) {
	return math.MaxUint64
}

type NetInInputs struct {
	HostIP        string
	HostPort      uint32
	ContainerIP   string
	ContainerPort uint32
}

type NetInOutputs struct {
	HostPort      uint32 `json:"host_port"`
	ContainerPort uint32 `json:"container_port"`
}

func (p *externalBinaryNetworker) NetIn(log lager.Logger, handle string, hostPort, containerPort uint32) (uint32, uint32, error) {
	containerIP, ok := p.configStore.Get(handle, gardener.ContainerIPKey)
	if !ok {
		return 0, 0, fmt.Errorf("cannot find container [%s]\n", handle)
	}

	inputs := NetInInputs{
		HostIP:        p.externalIP.String(),
		ContainerIP:   containerIP,
		HostPort:      hostPort,
		ContainerPort: containerPort,
	}
	outputs := NetInOutputs{}

	err := p.exec(log, "net-in", handle, inputs, &outputs)
	if err != nil {
		return 0, 0, err
	}

	err = kawasaki.AddPortMapping(log, p.configStore, handle, garden.PortMapping{
		HostPort:      outputs.HostPort,
		ContainerPort: outputs.ContainerPort,
	})
	if err != nil {
		return 0, 0, err
	}

	return outputs.HostPort, outputs.ContainerPort, err
}

type NetOutInputs struct {
	ContainerIP string            `json:"container_ip"`
	NetOutRule  garden.NetOutRule `json:"netout_rule"`
}

func (p *externalBinaryNetworker) NetOut(log lager.Logger, handle string, rule garden.NetOutRule) error {
	containerIP, ok := p.configStore.Get(handle, gardener.ContainerIPKey)
	if !ok {
		return fmt.Errorf("cannot find container [%s]\n", handle)
	}

	inputs := NetOutInputs{
		ContainerIP: containerIP,
		NetOutRule:  rule,
	}

	err := p.exec(log, "net-out", handle, inputs, nil)
	if err != nil {
		return err
	}

	return nil
}

type BulkNetOutInputs struct {
	ContainerIP string              `json:"container_ip"`
	NetOutRules []garden.NetOutRule `json:"netout_rules"`
}

func (p *externalBinaryNetworker) BulkNetOut(log lager.Logger, handle string, rules []garden.NetOutRule) error {
	containerIP, ok := p.configStore.Get(handle, gardener.ContainerIPKey)
	if !ok {
		return fmt.Errorf("cannot find container [%s]\n", handle)
	}

	inputs := BulkNetOutInputs{
		ContainerIP: containerIP,
		NetOutRules: rules,
	}

	return p.exec(log, "bulk-net-out", handle, inputs, nil)
}

func (p *externalBinaryNetworker) exec(log lager.Logger, action, handle string,
	inputData interface{}, outputData interface{}) error {

	stdinBytes, err := json.Marshal(inputData)
	if err != nil {
		return err
	}

	args := append(p.extraArg, "--action", action, "--handle", handle)
	cmd := exec.Command(p.path, args...)
	stdout := &bytes.Buffer{}
	cmd.Stdout = stdout
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	cmd.Stdin = bytes.NewReader(stdinBytes)

	err = p.commandRunner.Run(cmd)

	logData := lager.Data{"action": action, "stdin": string(stdinBytes), "stderr": stderr.String(), "stdout": stdout.String()}
	if err != nil {
		log.Error("external-networker-result", err, logData)
		return fmt.Errorf("external networker encountered an error running '%s' action: %s", action, err)
	}

	if outputData != nil && stdout.Len() > 0 {
		err = json.Unmarshal(stdout.Bytes(), outputData)
		if err != nil {
			log.Error("external-networker-result", err, logData)
			return fmt.Errorf("unmarshaling result from external networker: %s", err)
		}
	}

	if stderr.Len() > 0 {
		log.Info("external-networker-result", lager.Data{"stderr": stderr.String()})
	}

	log.Debug("external-networker-result", logData)

	return nil
}

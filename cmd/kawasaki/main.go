package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/cloudfoundry-incubator/cf-debug-server"
	"github.com/cloudfoundry-incubator/cf-lager"
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/dns"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/factory"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/iptables"
	"github.com/cloudfoundry-incubator/guardian/pkg/vars"
	"github.com/cloudfoundry/gunk/command_runner/linux_command_runner"
	"github.com/docker/docker/pkg/reexec"
	"github.com/opencontainers/specs/specs-go"
	"github.com/pivotal-golang/lager"
)

func init() {
	if reexec.Init() {
		os.Exit(0)
	}
}

func main() {
	cf_lager.AddFlags(flag.CommandLine)
	logger, _ := cf_lager.New("kawasaki")

	logFile := os.Getenv("GARDEN_LOG_FILE")
	logFileHandle, err := os.Create(logFile)
	if err != nil {
		panic(err)
	}

	logger.RegisterSink(lager.NewWriterSink(logFileHandle, lager.DEBUG))

	defer func() {
		if err := recover(); err != nil {
			logger.Fatal("panicked", fmt.Errorf("%#v", err))
		}
	}()

	state := specs.State{}
	if err := json.NewDecoder(os.Stdin).Decode(&state); err != nil {
		panic(err)
	}

	cf_debug_server.AddFlags(flag.CommandLine)

	var config kawasaki.NetworkConfig
	flag.StringVar(&config.HostIntf, "host-interface", "", "the host interface to create")
	flag.StringVar(&config.ContainerIntf, "container-interface", "", "the container interface to create")
	flag.StringVar(&config.BridgeName, "bridge-interface", "", "the bridge interface to create or use")
	flag.StringVar(&config.IPTablePrefix, "iptable-prefix", "", "the iptable chain prefix")
	flag.StringVar(&config.IPTableInstance, "iptable-instance", "", "the iptable instance to add rules to")
	flag.IntVar(&config.Mtu, "mtu", 1500, "the mtu")
	flag.Var(&IPValue{&config.BridgeIP}, "bridge-ip", "the IP address of the bridge interface")
	flag.Var(&IPValue{&config.ExternalIP}, "external-ip", "the IP address of the host interface")
	flag.Var(&IPValue{&config.ContainerIP}, "container-ip", "the IP address of the container interface")
	flag.Var(&vars.IPList{List: &config.DNSServers}, "dns-server", "the IP address(s) of DNS servers to unconditionally use")
	subnet := flag.String("subnet", "", "subnet of the bridge")
	flag.Parse()

	_, config.Subnet, err = net.ParseCIDR(*subnet)
	if err != nil {
		panic(err)
	}

	logger = logger.Session("hook", lager.Data{
		"config": config,
		"pid":    state.Pid,
	})

	logger.Info("start")

	configurer := factory.NewDefaultConfigurer(iptables.New(linux_command_runner.New(), config.IPTablePrefix))
	if err := configurer.Apply(logger, config, fmt.Sprintf("/proc/%d/ns/net", state.Pid)); err != nil {
		panic(err)
	}

	dnsResolvConfigurer := wireDNSResolvConfigurer(state, config)
	if err := dnsResolvConfigurer.Configure(logger); err != nil {
		panic(err)
	}
}

func extractRootIds(bndl *goci.Bndl) (int, int) {
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

func wireDNSResolvConfigurer(state specs.State, config kawasaki.NetworkConfig) *dns.ResolvConfigurer {
	bundleLoader := &goci.BndlLoader{}
	bndl, err := bundleLoader.Load(state.BundlePath)
	if err != nil {
		panic(err)
	}

	rootUid, rootGid := extractRootIds(bndl)

	configurer := &dns.ResolvConfigurer{
		HostsFileCompiler: &dns.HostsFileCompiler{
			Handle: state.ID,
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

type IPValue struct {
	*net.IP
}

func (i IPValue) String() string {
	return i.IP.String()
}

func (i IPValue) Set(s string) error {
	*i.IP = net.ParseIP(s)
	return nil
}

func (i IPValue) Get() interface{} {
	return i.IP
}

type CIDRValue struct {
	IPNet **net.IPNet
}

func (c CIDRValue) String() string {
	return (*c.IPNet).String()
}

func (c CIDRValue) Get() interface{} {
	return *c.IPNet
}

func (c CIDRValue) Set(s string) error {
	var err error
	_, *c.IPNet, err = net.ParseCIDR(s)
	return err
}

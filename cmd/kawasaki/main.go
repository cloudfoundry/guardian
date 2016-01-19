package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/cloudfoundry-incubator/cf-debug-server"
	"github.com/cloudfoundry-incubator/cf-lager"
	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/configure"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/iptables"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/netns"
	"github.com/cloudfoundry-incubator/guardian/logging"
	"github.com/cloudfoundry/gunk/command_runner/linux_command_runner"
	"github.com/opencontainers/specs"
	"github.com/pivotal-golang/lager"

	"github.com/cloudfoundry-incubator/guardian/kawasaki/devices"
)

func main() {
	state := specs.State{}
	if err := json.NewDecoder(os.Stdin).Decode(&state); err != nil {
		panic(err)
	}

	cf_debug_server.AddFlags(flag.CommandLine)
	cf_lager.AddFlags(flag.CommandLine)

	var config kawasaki.NetworkConfig
	flag.StringVar(&config.HostIntf, "host-interface", "", "the host interface to create")
	flag.StringVar(&config.ContainerIntf, "container-interface", "", "the container interface to create")
	flag.StringVar(&config.BridgeName, "bridge-interface", "", "the bridge interface to create or use")
	flag.StringVar(&config.IPTableChain, "iptable-chain", "", "the iptable chain to add rules to")
	flag.IntVar(&config.Mtu, "mtu", 1500, "the mtu")
	flag.Var(&IPValue{&config.BridgeIP}, "bridge-ip", "the IP address of the bridge interface")
	flag.Var(&IPValue{&config.ExternalIP}, "external-ip", "the IP address of the host interface")
	flag.Var(&IPValue{&config.ContainerIP}, "container-ip", "the IP address of the container interface")
	tag := flag.String("tag", "", "a uniquness tag for iptable chains")
	subnet := flag.String("subnet", "", "subnet of the bridge")
	flag.Parse()

	var err error
	_, config.Subnet, err = net.ParseCIDR(*subnet)
	if err != nil {
		panic(err)
	}

	logger, _ := cf_lager.New("kawasaki")

	logger = logger.Session("hook", lager.Data{
		"config": config,
		"pid":    state.Pid,
	})

	logger.Info("start")

	iptablesMgr := wireIptables(logger, *tag, true)
	configurer := wireConfigurer(logger, iptablesMgr)

	if err := configurer.Apply(logger, config, fmt.Sprintf("/proc/%d/ns/net", state.Pid)); err != nil {
		panic(err)
	}
}

func wireConfigurer(log lager.Logger, iptablesMgr *iptables.Manager) kawasaki.Configurer {
	hostConfigurer := &configure.Host{
		Veth:   &devices.VethCreator{},
		Link:   &devices.Link{Name: "guardian"},
		Bridge: &devices.Bridge{},
		Logger: log.Session("network-host-configurer"),
	}

	containerCfgApplier := &configure.Container{
		Logger: log.Session("network-container-configurer"),
		Link:   &devices.Link{Name: "guardian"},
	}

	configurer := kawasaki.NewConfigurer(
		hostConfigurer,
		containerCfgApplier,
		iptablesMgr,
		&netns.Execer{},
	)

	return configurer
}

func wireIptables(logger lager.Logger, tag string, allowHostAccess bool) *iptables.Manager {
	runner := &logging.Runner{CommandRunner: linux_command_runner.New(), Logger: logger.Session("iptables-runner")}

	filterConfig := iptables.FilterConfig{
		AllowHostAccess: allowHostAccess,
		InputChain:      fmt.Sprintf("g-%s-input", tag),
		ForwardChain:    fmt.Sprintf("g-%s-forward", tag),
		DefaultChain:    fmt.Sprintf("g-%s-default", tag),
	}

	natConfig := iptables.NATConfig{
		PreroutingChain:  fmt.Sprintf("g-%s-prerouting", tag),
		PostroutingChain: fmt.Sprintf("g-%s-postrouting", tag),
	}

	return iptables.NewManager(
		filterConfig,
		natConfig,
		runner,
		logger,
	)
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

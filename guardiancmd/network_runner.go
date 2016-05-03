package guardiancmd

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/factory"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/iptables"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/ports"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/subnets"
	"github.com/cloudfoundry-incubator/guardian/logging"
	"github.com/cloudfoundry-incubator/guardian/netplugin"
	"github.com/cloudfoundry-incubator/guardian/properties"
	"github.com/cloudfoundry/gunk/command_runner/linux_command_runner"
	"github.com/pivotal-golang/lager"
)

type NetworkCommand struct {
	Pool CIDRFlag `long:"network-pool" default:"10.254.0.0/22" description:"Network range to use for dynamically allocated container subnets."`

	AllowHostAccess bool       `long:"allow-host-access" description:"Allow network access to the host machine."`
	DenyNetworks    []CIDRFlag `long:"deny-network"      description:"Network ranges to which traffic from containers will be denied. Can be specified multiple times."`
	AllowNetworks   []CIDRFlag `long:"allow-network"     description:"Network ranges to which traffic from containers will be allowed. Can be specified multiple times."`

	DNSServers []IPFlag `long:"dns-server" description:"DNS server IP address to use instead of automatically determined servers. Can be specified multiple times."`

	ExternalIP    IPFlag `long:"external-ip"                     description:"IP address to use to reach container's mapped ports. Autodetected if not specified."`
	PortPoolStart uint32 `long:"port-pool-start" default:"60000" description:"Start of the ephemeral port range used for mapped container ports."`
	PortPoolSize  uint32 `long:"port-pool-size"  default:"5000"  description:"Size of the port pool used for mapped container ports."`

	Mtu int `long:"mtu" default:"1500" description:"MTU size for container network interfaces."`

	Plugin          FileFlag `long:"network-plugin"           description:"Path to network plugin binary."`
	PluginExtraArgs []string `long:"network-plugin-extra-arg" description:"Extra argument to pass to the network plugin. Can be specified multiple times."`
}

type NetworkRunner struct {
	networker gardener.Networker

	*NetworkCommand
	GetPropertyManager func() (*properties.Manager, error)
	Logger             LagerFlag
	ServerTag          string
	KawasakiBin        FileFlag
}

func (r *NetworkRunner) GetNetworker() (gardener.Networker, error) {
	if r.networker == nil {
		return nil, fmt.Errorf("networker not initialized")
	}
	return r.networker, nil
}

func (runner *NetworkRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger, _ := runner.Logger.Logger("networker")
	propManager, err := runner.GetPropertyManager()
	if err != nil {
		return err
	}

	networker, err := runner.NetworkCommand.wire(
		logger,
		propManager,
		runner.ServerTag,
		runner.KawasakiBin,
	)
	if err != nil {
		logger.Error("failed-to-wire-networker", err)
		return err
	}

	runner.networker = networker

	close(ready)
	logger.Info("ready")

	<-signals

	return nil
}

func (netcmd *NetworkCommand) wire(
	log lager.Logger,
	propManager kawasaki.ConfigStore,
	serverTag string,
	kawasakiBin FileFlag,
) (gardener.Networker, error) {

	interfacePrefix := fmt.Sprintf("w%s", serverTag)
	chainPrefix := fmt.Sprintf("w-%s-", serverTag)

	var denyNetworksList []string
	for _, network := range netcmd.DenyNetworks {
		denyNetworksList = append(denyNetworksList, network.String())
	}

	externalIP, err := defaultExternalIP(netcmd.ExternalIP)
	if err != nil {
		return nil, err
	}

	dnsServers := make([]net.IP, len(netcmd.DNSServers))
	for i, ip := range netcmd.DNSServers {
		dnsServers[i] = ip.IP()
	}

	var networkHookers []kawasaki.NetworkHooker
	if netcmd.Plugin.Path() != "" {
		networkHookers = append(networkHookers, netplugin.New(netcmd.Plugin.Path(), netcmd.PluginExtraArgs...))
	}

	iptRunner := &logging.Runner{CommandRunner: linux_command_runner.New(), Logger: log.Session("iptables-runner")}
	ipTables := iptables.New(iptRunner, chainPrefix)
	ipTablesStarter := iptables.NewStarter(ipTables, netcmd.AllowHostAccess, interfacePrefix, denyNetworksList)
	if err := ipTablesStarter.Start(); err != nil {
		return nil, fmt.Errorf("iptables starter: %s", err)
	}

	idGenerator := kawasaki.NewSequentialIDGenerator(time.Now().UnixNano())

	portPool, err := ports.NewPool(
		netcmd.PortPoolStart,
		netcmd.PortPoolSize,
		ports.State{})
	if err != nil {
		return nil, fmt.Errorf("invalid pool range: %s", err)
	}

	kawasakiNetworker := kawasaki.New(
		kawasakiBin.Path(),
		kawasaki.SpecParserFunc(kawasaki.ParseSpec),
		subnets.NewPool(netcmd.Pool.CIDR()),
		kawasaki.NewConfigCreator(idGenerator, interfacePrefix, chainPrefix, externalIP, dnsServers, netcmd.Mtu),
		propManager,
		factory.NewDefaultConfigurer(iptables.New(linux_command_runner.New(), chainPrefix)),
		portPool,
		iptables.NewPortForwarder(ipTables),
		iptables.NewFirewallOpener(ipTables),
	)

	networker := &kawasaki.CompositeNetworker{
		Networker:  kawasakiNetworker,
		ExtraHooks: networkHookers,
	}

	return networker, nil
}

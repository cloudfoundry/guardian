package iptables

import (
	"fmt"
	"net"
	"os/exec"

	"github.com/pivotal-golang/lager"
)

type InstanceChainCreator struct {
	iptables *IPTables
}

func NewInstanceChainCreator(iptables *IPTables) *InstanceChainCreator {
	return &InstanceChainCreator{
		iptables: iptables,
	}
}

func (cc *InstanceChainCreator) Create(logger lager.Logger, instanceId, bridgeName string, ip net.IP, network *net.IPNet) error {
	instanceChain := cc.iptables.instanceChain(instanceId)

	commands := []*exec.Cmd{
		// Create nat instance chain
		exec.Command("iptables", "--wait", "--table", "nat", "-N", instanceChain),
		// Bind nat instance chain to nat prerouting chain
		exec.Command("iptables", "--wait", "--table", "nat", "-A", cc.iptables.preroutingChain, "--jump", instanceChain),
		// Enable NAT for traffic coming from containers
		exec.Command("sh", "-c", fmt.Sprintf(
			`(iptables --wait --table nat -S %s | grep "\-j MASQUERADE\b" | grep -q -F -- "-s %s") || iptables --wait --table nat -A %s --source %s ! --destination %s --jump MASQUERADE`,
			cc.iptables.postroutingChain, network.String(), cc.iptables.postroutingChain,
			network.String(), network.String(),
		)),

		// Create filter instance chain
		exec.Command("iptables", "--wait", "-N", instanceChain),
		// Allow intra-subnet traffic (Linux ethernet bridging goes through ip stack)
		exec.Command("iptables", "--wait", "-A", instanceChain, "-s", network.String(), "-d", network.String(), "-j", "ACCEPT"),
		// Otherwise, use the default filter chain
		exec.Command("iptables", "--wait", "-A", instanceChain, "--goto", cc.iptables.defaultChain),
		// Bind filter instance chain to filter forward chain
		exec.Command("iptables", "--wait", "-I", cc.iptables.forwardChain, "2", "--in-interface", bridgeName, "--source", ip.String(), "--goto", instanceChain),
	}

	for _, cmd := range commands {
		if err := cc.iptables.run("create-instance-chains", cmd); err != nil {
			return err
		}
	}

	return nil
}

func (cc *InstanceChainCreator) Destroy(logger lager.Logger, instanceId string) error {
	instanceChain := cc.iptables.instanceChain(instanceId)

	commands := []*exec.Cmd{
		// Prune nat prerouting chain
		exec.Command("sh", "-c", fmt.Sprintf(
			`iptables --wait --table nat -S %s 2> /dev/null | grep "\-j %s\b" | sed -e "s/-A/-D/" | xargs --no-run-if-empty --max-lines=1 iptables --wait --table nat`,
			cc.iptables.preroutingChain, instanceChain,
		)),
		// Flush nat instance chain
		exec.Command("sh", "-c", fmt.Sprintf(`iptables --wait --table nat -F %s 2> /dev/null || true`, instanceChain)),
		// Delete nat instance chain
		exec.Command("sh", "-c", fmt.Sprintf(`iptables --wait --table nat -X %s 2> /dev/null || true`, instanceChain)),
		// Prune forward chain
		exec.Command("sh", "-c", fmt.Sprintf(
			`iptables --wait -S %s 2> /dev/null | grep "\-g %s\b" | sed -e "s/-A/-D/" | xargs --no-run-if-empty --max-lines=1 iptables --wait`,
			cc.iptables.forwardChain, instanceChain,
		)),
		// Flush instance chain
		exec.Command("sh", "-c", fmt.Sprintf("iptables --wait -F %s 2> /dev/null || true", instanceChain)),
		// Delete instance chain
		exec.Command("sh", "-c", fmt.Sprintf("iptables --wait -X %s 2> /dev/null || true", instanceChain)),
	}

	for _, cmd := range commands {
		if err := cc.iptables.run("destroy-instance-chains", cmd); err != nil {
			return err
		}
	}

	return nil
}

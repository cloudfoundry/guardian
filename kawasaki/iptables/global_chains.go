package iptables

import (
	"fmt"
	"os"
	"os/exec"
)

const SetupScript = `
	set -o xtrace
	set -o nounset
	set -o errexit
	shopt -s nullglob

	filter_input_chain="${GARDEN_IPTABLES_FILTER_INPUT_CHAIN}"
	filter_forward_chain="${GARDEN_IPTABLES_FILTER_FORWARD_CHAIN}"
	filter_default_chain="${GARDEN_IPTABLES_FILTER_DEFAULT_CHAIN}"
	filter_instance_prefix="${GARDEN_IPTABLES_FILTER_INSTANCE_PREFIX}"
	nat_prerouting_chain="${GARDEN_IPTABLES_NAT_PREROUTING_CHAIN}"
	nat_postrouting_chain="${GARDEN_IPTABLES_NAT_POSTROUTING_CHAIN}"
	nat_instance_prefix="${GARDEN_IPTABLES_NAT_INSTANCE_PREFIX}"
	iptables_bin="${GARDEN_IPTABLES_BIN}"

	function teardown_deprecated_rules() {
		# Remove jump to garden-dispatch from INPUT
		rules=$(${iptables_bin} -w -S INPUT 2> /dev/null) || true
		echo "$rules" |
		grep " -j garden-dispatch" |
		sed -e "s/-A/-D/" -e "s/\s\+\$//" |
		xargs --no-run-if-empty --max-lines=1 ${iptables_bin} -w

		# Remove jump to garden-dispatch from FORWARD
		rules=$(${iptables_bin} -w -S FORWARD 2> /dev/null) || true
		echo "$rules" |
		grep " -j garden-dispatch" |
		sed -e "s/-A/-D/" -e "s/\s\+\$//" |
		xargs --no-run-if-empty --max-lines=1 ${iptables_bin} -w

		# Prune garden-dispatch
		${iptables_bin} -w -F garden-dispatch 2> /dev/null || true

		# Delete garden-dispatch
		${iptables_bin} -w -X garden-dispatch 2> /dev/null || true
	}

	function teardown_filter() {
		teardown_deprecated_rules

		# Prune garden-forward chain
		rules=$(${iptables_bin} -w -S ${filter_forward_chain} 2> /dev/null) || true
		echo "$rules" |
		grep "\-g ${filter_instance_prefix}" |
		sed -e "s/-A/-D/" -e "s/\s\+\$//" |
		xargs --no-run-if-empty --max-lines=1 ${iptables_bin} -w

		# Prune per-instance chains
		rules=$(${iptables_bin} -w -S 2> /dev/null) || true
		echo "$rules" |
		grep "^-A ${filter_instance_prefix}" |
		sed -e "s/-A/-D/" -e "s/\s\+\$//" |
		xargs --no-run-if-empty --max-lines=1 ${iptables_bin} -w

		# Delete per-instance chains
		rules=$(${iptables_bin} -w -S 2> /dev/null) || true
		echo "$rules" |
		grep "^-N ${filter_instance_prefix}" |
		sed -e "s/-N/-X/" -e "s/\s\+\$//" |
		xargs --no-run-if-empty --max-lines=1 ${iptables_bin} -w || true

		# Remove jump to garden-forward from FORWARD
		rules=$(${iptables_bin} -w -S FORWARD 2> /dev/null) || true
		echo "$rules" |
		grep " -j ${filter_forward_chain}" |
		sed -e "s/-A/-D/" -e "s/\s\+\$//" |
		xargs --no-run-if-empty --max-lines=1 ${iptables_bin} -w || true

		${iptables_bin} -w -F ${filter_forward_chain} 2> /dev/null || true
		${iptables_bin} -w -F ${filter_default_chain} 2> /dev/null || true

		# Remove jump to filter input chain from INPUT
		rules=$(${iptables_bin} -w -S INPUT 2> /dev/null) || true
		echo "$rules" |
		grep " -j ${filter_input_chain}" |
		sed -e "s/-A/-D/" -e "s/\s\+\$//" |
		xargs --no-run-if-empty --max-lines=1 ${iptables_bin} -w || true

		# Empty and delete filter input chain
		${iptables_bin} -w -F ${filter_input_chain} 2> /dev/null || true
		${iptables_bin} -w -X ${filter_input_chain} 2> /dev/null || true
	}

	function setup_filter() {
		teardown_filter

		# Determine interface device to the outside
		default_interface=$(ip route show | grep default | cut -d' ' -f5 | head -1)

		# Create, or empty existing, filter input chain
		${iptables_bin} -w -N ${filter_input_chain} 2> /dev/null || ${iptables_bin} -w -F ${filter_input_chain}

		# Accept inbound packets if default interface is matched by filter prefix
		${iptables_bin} -w -I ${filter_input_chain} -i $default_interface --jump ACCEPT

		# Put connection tracking rule in filter input chain
		# to accept packets related to previously established connections
		${iptables_bin} -w -A ${filter_input_chain} -m conntrack --ctstate ESTABLISHED,RELATED --jump ACCEPT

		if [ "${GARDEN_IPTABLES_ALLOW_HOST_ACCESS}" != "true" ]; then
		${iptables_bin} -w -A ${filter_input_chain} --jump REJECT --reject-with icmp-host-prohibited
		else
		${iptables_bin} -w -A ${filter_input_chain} --jump ACCEPT
		fi

		# Forward input traffic via ${filter_input_chain}
		${iptables_bin} -w -A INPUT -i ${GARDEN_NETWORK_INTERFACE_PREFIX}+ --jump ${filter_input_chain}

		# Create or flush forward chain
		${iptables_bin} -w -N ${filter_forward_chain} 2> /dev/null || ${iptables_bin} -w -F ${filter_forward_chain}
		${iptables_bin} -w -A ${filter_forward_chain} -j DROP

		# Create or flush default chain
		${iptables_bin} -w -N ${filter_default_chain} 2> /dev/null || ${iptables_bin} -w -F ${filter_default_chain}

		# Always allow established connections to containers
		${iptables_bin} -w -A ${filter_default_chain} -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT

		# Forward outbound traffic via ${filter_forward_chain}
		${iptables_bin} -w -A FORWARD -i ${GARDEN_NETWORK_INTERFACE_PREFIX}+ --jump ${filter_forward_chain}

		# Forward inbound traffic immediately
		${iptables_bin} -w -I ${filter_forward_chain} -i $default_interface --jump ACCEPT
	}

	function teardown_nat() {
		# Prune prerouting chain
		rules=$(${iptables_bin} -w -t nat -S ${nat_prerouting_chain} 2> /dev/null) || true
		echo "$rules" |
		grep "\-j ${nat_instance_prefix}" |
		sed -e "s/-A/-D/" -e "s/\s\+\$//" |
		xargs --no-run-if-empty --max-lines=1 ${iptables_bin} -w -t nat

		# Prune per-instance chains
		rules=$(${iptables_bin} -w -t nat -S 2> /dev/null) || true
		echo "$rules" |
		grep "^-A ${nat_instance_prefix}" |
		sed -e "s/-A/-D/" -e "s/\s\+\$//" |
		xargs --no-run-if-empty --max-lines=1 ${iptables_bin} -w -t nat

		# Delete per-instance chains
		rules=$(${iptables_bin} -w -t nat -S 2> /dev/null) || true
		echo "$rules" |
		grep "^-N ${nat_instance_prefix}" |
		sed -e "s/-N/-X/" -e "s/\s\+\$//" |
		xargs --no-run-if-empty --max-lines=1 ${iptables_bin} -w -t nat || true

		# Flush prerouting chain
		${iptables_bin} -w -t nat -F ${nat_prerouting_chain} 2> /dev/null || true

		# Flush postrouting chain
		${iptables_bin} -w -t nat -F ${nat_postrouting_chain} 2> /dev/null || true
	}

	function setup_nat() {
		teardown_nat

		# Create prerouting chain
		${iptables_bin} -w -t nat -N ${nat_prerouting_chain} 2> /dev/null || true

		# Bind chain to PREROUTING
		(${iptables_bin} -w -t nat -S PREROUTING | grep -q "\-j ${nat_prerouting_chain}\b") ||
		${iptables_bin} -w -t nat -A PREROUTING \
		--jump ${nat_prerouting_chain}

		# Bind chain to OUTPUT (for traffic originating from same host)
		(${iptables_bin} -w -t nat -S OUTPUT | grep -q "\-j ${nat_prerouting_chain}\b") ||
		${iptables_bin} -w -t nat -A OUTPUT \
		--out-interface "lo" \
		--jump ${nat_prerouting_chain}

		# Create postrouting chain
		${iptables_bin} -w -t nat -N ${nat_postrouting_chain} 2> /dev/null || true

		# Bind chain to POSTROUTING
		(${iptables_bin} -w -t nat -S POSTROUTING | grep -q "\-j ${nat_postrouting_chain}\b") ||
		${iptables_bin} -w -t nat -A POSTROUTING \
		--jump ${nat_postrouting_chain}
	}

	case "${ACTION}" in
	setup)
	setup_filter
	setup_nat

	# Enable forwarding
	echo 1 > /proc/sys/net/ipv4/ip_forward
	;;
	teardown)
	teardown_filter
	teardown_nat
	;;
	*)
	echo "Unknown command: ${1}" 1>&2
	exit 1
	;;
	esac
`

type Starter struct {
	iptables                   *IPTablesController
	allowHostAccess            bool
	destroyContainersOnStartup bool
	nicPrefix                  string

	denyNetworks []string
}

func NewStarter(iptables *IPTablesController, allowHostAccess bool, nicPrefix string, denyNetworks []string, destroyContainersOnStartup bool) *Starter {
	return &Starter{
		iptables:                   iptables,
		allowHostAccess:            allowHostAccess,
		destroyContainersOnStartup: destroyContainersOnStartup,
		nicPrefix:                  nicPrefix,

		denyNetworks: denyNetworks,
	}
}

func (s Starter) Start() error {
	if s.destroyContainersOnStartup || !s.chainExists(s.iptables.inputChain) {
		cmd := exec.Command("bash", "-c", SetupScript)
		cmd.Env = []string{
			fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
			"ACTION=setup",
			fmt.Sprintf("GARDEN_IPTABLES_BIN=%s", s.iptables.binPath),
			fmt.Sprintf("GARDEN_IPTABLES_FILTER_INPUT_CHAIN=%s", s.iptables.inputChain),
			fmt.Sprintf("GARDEN_IPTABLES_FILTER_FORWARD_CHAIN=%s", s.iptables.forwardChain),
			fmt.Sprintf("GARDEN_IPTABLES_FILTER_DEFAULT_CHAIN=%s", s.iptables.defaultChain),
			fmt.Sprintf("GARDEN_IPTABLES_FILTER_INSTANCE_PREFIX=%s", s.iptables.instanceChainPrefix),
			fmt.Sprintf("GARDEN_IPTABLES_NAT_PREROUTING_CHAIN=%s", s.iptables.preroutingChain),
			fmt.Sprintf("GARDEN_IPTABLES_NAT_POSTROUTING_CHAIN=%s", s.iptables.postroutingChain),
			fmt.Sprintf("GARDEN_IPTABLES_NAT_INSTANCE_PREFIX=%s", s.iptables.instanceChainPrefix),
			fmt.Sprintf("GARDEN_NETWORK_INTERFACE_PREFIX=%s", s.nicPrefix),
			fmt.Sprintf("GARDEN_IPTABLES_ALLOW_HOST_ACCESS=%t", s.allowHostAccess),
		}

		if err := s.iptables.run("setup-global-chains", cmd); err != nil {
			return fmt.Errorf("setting up default chains: %s", err)
		}
	}

	if err := s.resetDenyNetworks(); err != nil {
		return err
	}

	for _, n := range s.denyNetworks {
		if err := s.iptables.appendRule(s.iptables.defaultChain, rejectRule(n)); err != nil {
			return err
		}
	}

	return nil
}

func (s Starter) chainExists(chainName string) bool {
	cmd := exec.Command(s.iptables.binPath, "-w", "-L", chainName)
	cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s", os.Getenv("PATH")))
	return s.iptables.run("checking-chain-exists", cmd) == nil
}

func (s Starter) resetDenyNetworks() error {
	cmd := exec.Command(s.iptables.binPath, "-w", "-F", s.iptables.defaultChain)
	cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s", os.Getenv("PATH")))
	if err := s.iptables.run("flushing-default-chain", cmd); err != nil {
		return err
	}

	cmd = exec.Command(s.iptables.binPath, "-w", "-A", s.iptables.defaultChain, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "--jump", "ACCEPT")
	cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s", os.Getenv("PATH")))
	if err := s.iptables.run("appending-default-chain", cmd); err != nil {
		return err
	}

	return nil
}

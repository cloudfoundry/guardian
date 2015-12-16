package iptables

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/cloudfoundry/gunk/command_runner"
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

	function teardown_deprecated_rules() {
		# Remove jump to garden-dispatch from INPUT
		iptables -w -S INPUT 2> /dev/null |
		grep " -j garden-dispatch" |
		sed -e "s/-A/-D/" -e "s/\s\+\$//" |
		xargs --no-run-if-empty --max-lines=1 iptables -w

		# Remove jump to garden-dispatch from FORWARD
		iptables -w -S FORWARD 2> /dev/null |
		grep " -j garden-dispatch" |
		sed -e "s/-A/-D/" -e "s/\s\+\$//" |
		xargs --no-run-if-empty --max-lines=1 iptables -w

		# Prune garden-dispatch
		iptables -w -F garden-dispatch 2> /dev/null || true

		# Delete garden-dispatch
		iptables -w -X garden-dispatch 2> /dev/null || true
	}

	function teardown_filter() {
		teardown_deprecated_rules

		# Prune garden-forward chain
		iptables -w -S ${filter_forward_chain} 2> /dev/null |
		grep "\-g ${filter_instance_prefix}" |
		sed -e "s/-A/-D/" -e "s/\s\+\$//" |
		xargs --no-run-if-empty --max-lines=1 iptables -w

		# Prune per-instance chains
		iptables -w -S 2> /dev/null |
		grep "^-A ${filter_instance_prefix}" |
		sed -e "s/-A/-D/" -e "s/\s\+\$//" |
		xargs --no-run-if-empty --max-lines=1 iptables -w

		# Delete per-instance chains
		iptables -w -S 2> /dev/null |
		grep "^-N ${filter_instance_prefix}" |
		sed -e "s/-N/-X/" -e "s/\s\+\$//" |
		xargs --no-run-if-empty --max-lines=1 iptables -w || true

		# Remove jump to garden-forward from FORWARD
		iptables -w -S FORWARD 2> /dev/null |
		grep " -j ${filter_forward_chain}" |
		sed -e "s/-A/-D/" -e "s/\s\+\$//" |
		xargs --no-run-if-empty --max-lines=1 iptables -w || true

		iptables -w -F ${filter_forward_chain} 2> /dev/null || true
		iptables -w -F ${filter_default_chain} 2> /dev/null || true

		# Remove jump to filter input chain from INPUT
		iptables -w -S INPUT 2> /dev/null |
		grep " -j ${filter_input_chain}" |
		sed -e "s/-A/-D/" -e "s/\s\+\$//" |
		xargs --no-run-if-empty --max-lines=1 iptables -w || true

		# Empty and delete filter input chain
		iptables -w -F ${filter_input_chain} 2> /dev/null || true
		iptables -w -X ${filter_input_chain} 2> /dev/null || true
	}

	function setup_filter() {
		teardown_filter

		# Determine interface device to the outside
		default_interface=$(ip route show | grep default | cut -d' ' -f5 | head -1)

		# Create, or empty existing, filter input chain
		iptables -w -N ${filter_input_chain} 2> /dev/null || iptables -w -F ${filter_input_chain}

		# Accept inbound packets if default interface is matched by filter prefix
		iptables -w -I ${filter_input_chain} -i $default_interface --jump ACCEPT

		# Put connection tracking rule in filter input chain
		# to accept packets related to previously established connections
		iptables -w -A ${filter_input_chain} -m conntrack --ctstate ESTABLISHED,RELATED --jump ACCEPT

		if [ "${GARDEN_IPTABLES_ALLOW_HOST_ACCESS}" != "true" ]; then
		iptables -w -A ${filter_input_chain} --jump REJECT --reject-with icmp-host-prohibited
		else
		iptables -w -A ${filter_input_chain} --jump ACCEPT
		fi

		# Forward input traffic via ${filter_input_chain}
		iptables -w -A INPUT -i ${GARDEN_NETWORK_INTERFACE_PREFIX}+ --jump ${filter_input_chain}

		# Create or flush forward chain
		iptables -w -N ${filter_forward_chain} 2> /dev/null || iptables -w -F ${filter_forward_chain}
		iptables -w -A ${filter_forward_chain} -j DROP

		# Create or flush default chain
		iptables -w -N ${filter_default_chain} 2> /dev/null || iptables -w -F ${filter_default_chain}

		# Always allow established connections to containers
		iptables -w -A ${filter_default_chain} -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT

		# Forward outbound traffic via ${filter_forward_chain}
		iptables -w -A FORWARD -i ${GARDEN_NETWORK_INTERFACE_PREFIX}+ --jump ${filter_forward_chain}

		# Forward inbound traffic immediately
		iptables -w -I ${filter_forward_chain} -i $default_interface --jump ACCEPT
	}

	function teardown_nat() {
		# Prune prerouting chain
		iptables -w -t nat -S ${nat_prerouting_chain} 2> /dev/null |
		grep "\-j ${nat_instance_prefix}" |
		sed -e "s/-A/-D/" -e "s/\s\+\$//" |
		xargs --no-run-if-empty --max-lines=1 iptables -w -t nat

		# Prune per-instance chains
		iptables -w -t nat -S 2> /dev/null |
		grep "^-A ${nat_instance_prefix}" |
		sed -e "s/-A/-D/" -e "s/\s\+\$//" |
		xargs --no-run-if-empty --max-lines=1 iptables -w -t nat

		# Delete per-instance chains
		iptables -w -t nat -S 2> /dev/null |
		grep "^-N ${nat_instance_prefix}" |
		sed -e "s/-N/-X/" -e "s/\s\+\$//" |
		xargs --no-run-if-empty --max-lines=1 iptables -w -t nat || true

		# Flush prerouting chain
		iptables -w -t nat -F ${nat_prerouting_chain} 2> /dev/null || true

		# Flush postrouting chain
		iptables -w -t nat -F ${nat_postrouting_chain} 2> /dev/null || true
	}

	function setup_nat() {
		teardown_nat

		# Create prerouting chain
		iptables -w -t nat -N ${nat_prerouting_chain} 2> /dev/null || true

		# Bind chain to PREROUTING
		(iptables -w -t nat -S PREROUTING | grep -q "\-j ${nat_prerouting_chain}\b") ||
		iptables -w -t nat -A PREROUTING \
		--jump ${nat_prerouting_chain}

		# Bind chain to OUTPUT (for traffic originating from same host)
		(iptables -w -t nat -S OUTPUT | grep -q "\-j ${nat_prerouting_chain}\b") ||
		iptables -w -t nat -A OUTPUT \
		--out-interface "lo" \
		--jump ${nat_prerouting_chain}

		# Create postrouting chain
		iptables -w -t nat -N ${nat_postrouting_chain} 2> /dev/null || true

		# Bind chain to POSTROUTING
		(iptables -w -t nat -S POSTROUTING | grep -q "\-j ${nat_postrouting_chain}\b") ||
		iptables -w -t nat -A POSTROUTING \
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
	runner      command_runner.CommandRunner
	fc          FilterConfig
	nc          NATConfig
	chainPrefix string
	nicPrefix   string
}

func NewStarter(runner command_runner.CommandRunner, fc FilterConfig, nc NATConfig, chainPrefix, nicPrefix string) *Starter {
	return &Starter{
		runner:      runner,
		fc:          fc,
		nc:          nc,
		chainPrefix: chainPrefix,
		nicPrefix:   nicPrefix,
	}
}

func (s Starter) Start() error {
	cmd := exec.Command("bash", "-c", SetupScript)
	cmd.Env = []string{
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
		"ACTION=setup",
		fmt.Sprintf("GARDEN_IPTABLES_FILTER_INPUT_CHAIN=%s", s.fc.InputChain),
		fmt.Sprintf("GARDEN_IPTABLES_FILTER_FORWARD_CHAIN=%s", s.fc.ForwardChain),
		fmt.Sprintf("GARDEN_IPTABLES_FILTER_DEFAULT_CHAIN=%s", s.fc.DefaultChain),
		fmt.Sprintf("GARDEN_IPTABLES_FILTER_INSTANCE_PREFIX=%s", s.chainPrefix),
		fmt.Sprintf("GARDEN_IPTABLES_NAT_PREROUTING_CHAIN=%s", s.nc.PreroutingChain),
		fmt.Sprintf("GARDEN_IPTABLES_NAT_POSTROUTING_CHAIN=%s", s.nc.PostroutingChain),
		fmt.Sprintf("GARDEN_IPTABLES_NAT_INSTANCE_PREFIX=%s", s.chainPrefix),
		fmt.Sprintf("GARDEN_NETWORK_INTERFACE_PREFIX=%s", s.nicPrefix),
		fmt.Sprintf("GARDEN_IPTABLES_ALLOW_HOST_ACCESS=%t", s.fc.AllowHostAccess),
	}

	return s.runner.Run(cmd)
}

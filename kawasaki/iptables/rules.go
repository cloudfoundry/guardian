package iptables

import (
	"fmt"

	"code.cloudfoundry.org/garden"
)

type iptablesFlags []string

func (flags iptablesFlags) Flags(chain string) []string {
	return flags
}

func natRule(destination string, destinationPort uint32, containerIP string, containerPort uint32, comment string) Rule {
	return iptablesFlags([]string{
		"--table", "nat",
		"--protocol", "tcp",
		"--destination", destination,
		"--destination-port", fmt.Sprintf("%d", destinationPort),
		"--jump", "DNAT",
		"--to-destination", fmt.Sprintf("%s:%d", containerIP, containerPort),
		"-m", "comment", "--comment", comment,
	})
}

func rejectRule(destination string) Rule {
	return iptablesFlags([]string{
		"--destination", destination,
		"--jump", "REJECT",
	})
}

type SingleFilterRule struct {
	Protocol garden.Protocol
	Networks *garden.IPRange
	Ports    *garden.PortRange
	ICMPs    *garden.ICMPControl
	Log      bool
	Handle   string
}

func (r SingleFilterRule) Flags(chain string) (params []string) {
	params = append(params, "--protocol", protocols[r.Protocol])

	network := r.Networks
	if network != nil {
		if network.Start != nil && network.End != nil {
			params = append(params, "-m", "iprange", "--dst-range", network.Start.String()+"-"+network.End.String())
		} else if network.Start != nil {
			params = append(params, "--destination", network.Start.String())
		} else if network.End != nil {
			params = append(params, "--destination", network.End.String())
		}
	}

	ports := r.Ports
	if ports != nil {
		if ports.End != ports.Start {
			params = append(params, "--destination-port", fmt.Sprintf("%d:%d", ports.Start, ports.End))
		} else {
			params = append(params, "--destination-port", fmt.Sprintf("%d", ports.Start))
		}
	}

	if r.ICMPs != nil {
		icmpType := fmt.Sprintf("%d", r.ICMPs.Type)
		if r.ICMPs.Code != nil {
			icmpType = fmt.Sprintf("%d/%d", r.ICMPs.Type, *r.ICMPs.Code)
		}

		params = append(params, "--icmp-type", icmpType)
	}

	if r.Log {
		params = append(params, "--goto", chain+"-log")
	} else {
		params = append(params, "--jump", "RETURN")
	}

	params = append(params, "-m", "comment", "--comment", r.Handle)

	return params
}

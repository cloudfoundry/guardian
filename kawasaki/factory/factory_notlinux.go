// +build !linux

package factory

import (
	"code.cloudfoundry.org/guardian/kawasaki"
	"code.cloudfoundry.org/guardian/kawasaki/iptables"
)

func NewDefaultConfigurer(ipt *iptables.IPTablesController) kawasaki.Configurer {
	panic("not supported on this platform")
}

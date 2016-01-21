// +build !linux

package factory

import (
	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/iptables"
)

func NewDefaultConfigurer(ipt *iptables.IPTables) kawasaki.Configurer {
	panic("not supported on this platform")
}

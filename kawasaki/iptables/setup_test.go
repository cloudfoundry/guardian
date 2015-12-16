package iptables_test

import (
	"fmt"
	"os"

	"github.com/cloudfoundry-incubator/guardian/kawasaki/iptables"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/cloudfoundry/gunk/command_runner/fake_command_runner/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Setup", func() {
	It("runs the setup script, passing the environment variables", func() {
		fakeRunner := fake_command_runner.New()
		iptables.NewStarter(fakeRunner, iptables.FilterConfig{
			InputChain:      "the-filter-input-chain",
			ForwardChain:    "the-filter-forward-chain",
			DefaultChain:    "the-filter-default-chain",
			AllowHostAccess: true,
		}, iptables.NATConfig{
			PreroutingChain:  "the-nat-prerouting-chain",
			PostroutingChain: "the-nat-postrouting-chain",
		}, "the-chain-prefix", "the-nic-prefix").Start()

		Expect(fakeRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
			Path: "bash",
			Args: []string{"-c", iptables.SetupScript},
			Env: []string{
				fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
				"ACTION=setup",

				"GARDEN_IPTABLES_FILTER_INPUT_CHAIN=the-filter-input-chain",
				"GARDEN_IPTABLES_FILTER_FORWARD_CHAIN=the-filter-forward-chain",
				"GARDEN_IPTABLES_FILTER_DEFAULT_CHAIN=the-filter-default-chain",
				"GARDEN_IPTABLES_FILTER_INSTANCE_PREFIX=the-chain-prefix",
				"GARDEN_IPTABLES_NAT_PREROUTING_CHAIN=the-nat-prerouting-chain",
				"GARDEN_IPTABLES_NAT_POSTROUTING_CHAIN=the-nat-postrouting-chain",
				"GARDEN_IPTABLES_NAT_INSTANCE_PREFIX=the-chain-prefix",
				"GARDEN_NETWORK_INTERFACE_PREFIX=the-nic-prefix",
				"GARDEN_IPTABLES_ALLOW_HOST_ACCESS=true",
			},
		}))
	})
})

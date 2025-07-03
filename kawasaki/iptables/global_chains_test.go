package iptables_test

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	. "code.cloudfoundry.org/commandrunner/fake_command_runner/matchers"
	"code.cloudfoundry.org/guardian/kawasaki/iptables"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Setup", func() {
	var (
		fakeRunner                 *fake_command_runner.FakeCommandRunner
		denyNetworks               []string
		destroyContainersOnStartup bool
		starter                    *iptables.Starter
	)

	JustBeforeEach(func() {
		fakeLocksmith := NewFakeLocksmith()
		starter = iptables.NewStarter(
			iptables.New("/sbin/iptables", "/sbin/iptables-restore", fakeRunner, fakeLocksmith, "prefix-"),
			true,
			"the-nic-prefix",
			denyNetworks,
			destroyContainersOnStartup,
			lagertest.NewTestLogger("global_chains_test"),
		)
	})

	itSetsUpGlobalChains := func() {
		Expect(fakeRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
			Path: "bash",
			Args: []string{"-c", iptables.SetupScript},
			Env: []string{
				fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
				"ACTION=setup",

				"GARDEN_IPTABLES_BIN=/sbin/iptables",
				"GARDEN_IPTABLES_FILTER_INPUT_CHAIN=prefix-input",
				"GARDEN_IPTABLES_FILTER_FORWARD_CHAIN=prefix-forward",
				"GARDEN_IPTABLES_FILTER_DEFAULT_CHAIN=prefix-default",
				"GARDEN_IPTABLES_FILTER_INSTANCE_PREFIX=prefix-instance-",
				"GARDEN_IPTABLES_NAT_PREROUTING_CHAIN=prefix-prerouting",
				"GARDEN_IPTABLES_NAT_POSTROUTING_CHAIN=prefix-postrouting",
				"GARDEN_IPTABLES_NAT_INSTANCE_PREFIX=prefix-instance-",
				"GARDEN_NETWORK_INTERFACE_PREFIX=the-nic-prefix",
				"GARDEN_IPTABLES_ALLOW_HOST_ACCESS=true",
			},
		}))
	}

	Describe("Global chains setup", func() {

		Context("when destroy_containers_on_startup is set to true", func() {
			BeforeEach(func() {
				destroyContainersOnStartup = true
			})

			It("runs the setup script, passing the environment variables", func() {
				Expect(starter.Start()).To(Succeed())

				itSetsUpGlobalChains()
			})
		})

	})
})

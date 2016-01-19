package iptables_test

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/cloudfoundry-incubator/guardian/kawasaki/iptables"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/cloudfoundry/gunk/command_runner/fake_command_runner/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Setup", func() {
	var (
		fakeRunner   *fake_command_runner.FakeCommandRunner
		denyNetworks []string
		starter      *iptables.Starter
	)

	BeforeEach(func() {
		fakeRunner = fake_command_runner.New()
	})

	JustBeforeEach(func() {
		starter = iptables.NewStarter(fakeRunner, iptables.FilterConfig{
			InputChain:      "the-filter-input-chain",
			ForwardChain:    "the-filter-forward-chain",
			DefaultChain:    "the-filter-default-chain",
			DenyNetworks:    denyNetworks,
			AllowHostAccess: true,
		}, iptables.NATConfig{
			PreroutingChain:  "the-nat-prerouting-chain",
			PostroutingChain: "the-nat-postrouting-chain",
		}, "the-chain-prefix", "the-nic-prefix")
	})

	It("runs the setup script, passing the environment variables", func() {
		Expect(starter.Start()).To(Succeed())

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

	Context("when running the setup script fails", func() {
		BeforeEach(func() {
			fakeRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "bash",
				Args: []string{"-c", iptables.SetupScript},
			}, func(_ *exec.Cmd) error {
				return fmt.Errorf("oh no!")
			})
		})

		It("returns the error", func() {
			Expect(starter.Start()).To(MatchError(ContainSubstring("oh no!")))
		})
	})

	Context("when denyNetworks is set", func() {
		BeforeEach(func() {
			denyNetworks = []string{"1.2.3.4/11", "5.6.7.8/33"}
		})

		It("runs IPTables to deny networks", func() {
			Expect(starter.Start()).To(Succeed())

			Expect(fakeRunner).To(HaveExecutedSerially(
				fake_command_runner.CommandSpec{
					Path: "bash",
					Args: []string{"-c", iptables.SetupScript},
				},
				fake_command_runner.CommandSpec{
					Path: "/sbin/iptables",
					Args: []string{"-w", "-A", "the-filter-default-chain", "-d", "1.2.3.4/11", "-j", "REJECT"},
				},
				fake_command_runner.CommandSpec{
					Path: "/sbin/iptables",
					Args: []string{"-w", "-A", "the-filter-default-chain", "-d", "5.6.7.8/33", "-j", "REJECT"},
				},
			))
		})

		Context("when the first command fails", func() {
			BeforeEach(func() {
				fakeRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "/sbin/iptables",
				}, func(_ *exec.Cmd) error {
					return fmt.Errorf("oh banana error!")
				})
			})

			It("returns the error", func() {
				Expect(starter.Start()).To(MatchError(ContainSubstring("oh banana error!")))
			})

			It("does not try to apply the rest of the deny rules", func() {
				starter.Start()

				Expect(fakeRunner.ExecutedCommands()).To(HaveLen(2))
			})
		})
	})
})

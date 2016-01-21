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
		starter = iptables.NewStarter(
			iptables.New(fakeRunner, "prefix-"),
			true,
			"the-nic-prefix",
			denyNetworks,
		)
	})

	It("runs the setup script, passing the environment variables", func() {
		Expect(starter.Start()).To(Succeed())

		Expect(fakeRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
			Path: "bash",
			Args: []string{"-c", iptables.SetupScript},
			Env: []string{
				fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
				"ACTION=setup",

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
	})

	Context("when running the setup script fails", func() {
		BeforeEach(func() {
			fakeRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "bash",
				Args: []string{"-c", iptables.SetupScript},
			}, func(cmd *exec.Cmd) error {
				cmd.Stderr.Write([]byte("oh no!"))
				return fmt.Errorf("exit status something")
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
					Args: []string{"-w", "-A", "prefix-default", "--destination", "1.2.3.4/11", "--jump", "REJECT"},
				},
				fake_command_runner.CommandSpec{
					Path: "/sbin/iptables",
					Args: []string{"-w", "-A", "prefix-default", "--destination", "5.6.7.8/33", "--jump", "REJECT"},
				},
			))
		})

		Context("when the first command fails", func() {
			BeforeEach(func() {
				fakeRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "/sbin/iptables",
				}, func(cmd *exec.Cmd) error {
					cmd.Stderr.Write([]byte("oh banana error!"))
					return fmt.Errorf("exit status something")
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

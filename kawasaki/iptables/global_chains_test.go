package iptables_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	. "code.cloudfoundry.org/commandrunner/fake_command_runner/matchers"
	"code.cloudfoundry.org/guardian/kawasaki/iptables"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Setup", func() {
	var (
		fakeRunner                 *fake_command_runner.FakeCommandRunner
		denyNetworks               []string
		destroyContainersOnStartup bool
		starter                    *iptables.Starter
	)

	BeforeEach(func() {
		fakeRunner = fake_command_runner.New()
		destroyContainersOnStartup = false
	})

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

	itDoesNotSetUpGlobalChains := func() {
		Expect(fakeRunner).NotTo(HaveExecutedSerially(fake_command_runner.CommandSpec{
			Path: "bash",
			Args: []string{"-c", iptables.SetupScript},
		}))
	}

	itRejectsNetwork := func(network string) {
		Expect(fakeRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
			Path: "/sbin/iptables",
			Args: []string{"-w", "-A", "prefix-default", "--destination", network, "--jump", "REJECT"},
		}))
	}

	itFlushesChain := func(chain string) {
		Expect(fakeRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
			Path: "/sbin/iptables",
			Args: []string{"-w", "-F", chain},
		}))
	}

	itAppendsRule := func(chain string, args ...string) {
		Expect(fakeRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
			Path: "/sbin/iptables",
			Args: append([]string{"-w", "-A", chain}, args...),
		}))
	}

	Describe("Global chains setup", func() {
		Context("when the input chain does not exist", func() {
			BeforeEach(func() {
				fakeRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "/sbin/iptables",
					Args: []string{"-w", "-n", "-L", "prefix-input"},
				}, func(_ *exec.Cmd) error {
					return errors.New("exit status 1")
				})
			})

			It("runs the setup script, passing the environment variables", func() {
				Expect(starter.Start()).To(Succeed())

				itSetsUpGlobalChains()
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

			Describe("DenyNetwork rules", func() {
				BeforeEach(func() {
					denyNetworks = []string{"1.2.3.4/11", "5.6.7.8/33"}
				})

				It("runs IPTables to deny networks", func() {
					Expect(starter.Start()).To(Succeed())

					itRejectsNetwork("1.2.3.4/11")
					itRejectsNetwork("5.6.7.8/33")
				})

				Context("when the first command fails", func() {
					BeforeEach(func() {
						fakeRunner.WhenRunning(fake_command_runner.CommandSpec{
							Path: "/sbin/iptables",
							Args: []string{"-w", "-A", "prefix-default", "--destination", "1.2.3.4/11", "--jump", "REJECT"},
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

						Expect(fakeRunner.ExecutedCommands()).To(HaveLen(5))
					})
				})
			})
		})

		Context("when destroy_containers_on_startup is set to true", func() {
			BeforeEach(func() {
				destroyContainersOnStartup = true
			})

			It("runs the setup script, passing the environment variables", func() {
				Expect(starter.Start()).To(Succeed())

				itSetsUpGlobalChains()
			})
		})

		Context("when the input chain exists", func() {
			BeforeEach(func() {
				fakeRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "/sbin/iptables",
					Args: []string{"-w", "-n", "-L", "prefix-input"},
				}, func(_ *exec.Cmd) error {
					return nil
				})
			})

			It("does not run the setup script", func() {
				Expect(starter.Start()).To(Succeed())

				itDoesNotSetUpGlobalChains()
			})
		})

		Describe("DenyNetwork rules", func() {
			Context("and previous deny networks are applied", func() {
				BeforeEach(func() {
					denyNetworks = []string{"4.3.2.1/11", "8.7.6.5/33"}
				})

				It("removes the old rules", func() {
					Expect(starter.Start()).To(Succeed())

					itFlushesChain("prefix-default")
					itAppendsRule("prefix-default", "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "--jump", "ACCEPT")
				})

				It("adds the new rules", func() {
					Expect(starter.Start()).To(Succeed())

					itRejectsNetwork("4.3.2.1/11")
					itRejectsNetwork("8.7.6.5/33")
				})

				Context("when resetting deny networks fail", func() {
					Context("when flushing the chain fails", func() {
						BeforeEach(func() {
							fakeRunner.WhenRunning(fake_command_runner.CommandSpec{
								Path: "/sbin/iptables",
								Args: []string{"-w", "-F", "prefix-default"},
							}, func(cmd *exec.Cmd) error {
								cmd.Stderr.Write([]byte("cannot-flush"))
								return errors.New("cannot-flush")
							})
						})

						It("returns the error", func() {
							err := starter.Start()
							Expect(err).To(MatchError(ContainSubstring("cannot-flush")))
						})
					})

					Context("when appending the default chain fails", func() {
						BeforeEach(func() {
							fakeRunner.WhenRunning(fake_command_runner.CommandSpec{
								Path: "/sbin/iptables",
								Args: []string{"-w", "-A", "prefix-default", "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "--jump", "ACCEPT"},
							}, func(cmd *exec.Cmd) error {
								cmd.Stderr.Write([]byte("cannot-apply-conntrack"))
								return errors.New("cannot-apply-conntrack")
							})
						})

						It("returns the error", func() {
							err := starter.Start()
							Expect(err).To(MatchError(ContainSubstring("cannot-apply-conntrack")))
						})
					})
				})
			})
		})
	})
})

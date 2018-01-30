package iptables_test

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	"code.cloudfoundry.org/guardian/kawasaki/iptables"
	fakes "code.cloudfoundry.org/guardian/kawasaki/iptables/iptablesfakes"
	"code.cloudfoundry.org/guardian/pkg/locksmith"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IPTables controller", func() {
	var (
		netnsName          string
		prefix             string
		iptablesController iptables.IPTables
		fakeLocksmith      *FakeLocksmith
		fakeRunner         *fake_command_runner.FakeCommandRunner
	)

	BeforeEach(func() {
		SetDefaultEventuallyTimeout(8 * time.Second)
		netnsName = fmt.Sprintf("ginkgo-netns-%d", GinkgoParallelNode())
		makeNamespace(netnsName)

		fakeRunner = fake_command_runner.New()
		fakeRunner.WhenRunning(fake_command_runner.CommandSpec{},
			func(cmd *exec.Cmd) error {
				if len(cmd.Args) >= 4 && cmd.Args[3] == "panic" {
					panic("ops")
				}
				return wrapCmdInNs(netnsName, cmd).Run()
			},
		)

		fakeLocksmith = NewFakeLocksmith()

		prefix = fmt.Sprintf("g-%d", GinkgoParallelNode())
		iptablesController = iptables.New("/sbin/iptables", "/sbin/iptables-restore", fakeRunner, fakeLocksmith, prefix)
	})

	AfterEach(func() {
		deleteNamespace(netnsName)
	})

	Describe("CreateChain", func() {
		It("creates the chain", func() {
			Expect(iptablesController.CreateChain("filter", "test-chain")).To(Succeed())
			runForStdout(wrapCmdInNs(netnsName, exec.Command("iptables", "-w", "-n", "-L", "test-chain")))
		})

		Context("when the table is nat", func() {
			It("creates the nat chain", func() {
				Expect(iptablesController.CreateChain("nat", "test-chain")).To(Succeed())
				runForStdout(wrapCmdInNs(netnsName, exec.Command("iptables", "-w", "-t", "nat", "-n", "-L", "test-chain")))
			})
		})

		Context("when the chain already exists", func() {
			BeforeEach(func() {
				Expect(iptablesController.CreateChain("nat", "test-chain")).To(Succeed())
			})

			It("returns an error", func() {
				Expect(iptablesController.CreateChain("nat", "test-chain")).NotTo(Succeed())
			})
		})
	})

	Describe("PrependRule", func() {
		It("prepends the rule", func() {
			fakeTCPRule := new(fakes.FakeRule)
			fakeTCPRule.FlagsReturns([]string{"--protocol", "tcp"})
			fakeUDPRule := new(fakes.FakeRule)
			fakeUDPRule.FlagsReturns([]string{"--protocol", "udp"})

			Expect(iptablesController.CreateChain("filter", "test-chain")).To(Succeed())

			Expect(iptablesController.PrependRule("test-chain", fakeTCPRule)).To(Succeed())
			Expect(iptablesController.PrependRule("test-chain", fakeUDPRule)).To(Succeed())

			cmd := runForStdout(wrapCmdInNs(netnsName, exec.Command("iptables", "-w", "-S", "test-chain")))
			Expect(cmd).To(ContainSubstring("-A test-chain -p udp\n-A test-chain -p tcp"))
		})

		It("returns an error when the chain does not exist", func() {
			fakeRule := new(fakes.FakeRule)
			fakeRule.FlagsReturns([]string{})

			Expect(iptablesController.PrependRule("test-chain", fakeRule)).NotTo(Succeed())
		})
	})

	Describe("BulkPrependRules", func() {
		It("appends the rules", func() {
			fakeTCPRule := new(fakes.FakeRule)
			fakeTCPRule.FlagsReturns([]string{"--protocol", "tcp"})
			fakeUDPRule := new(fakes.FakeRule)
			fakeUDPRule.FlagsReturns([]string{"--protocol", "udp"})

			Expect(iptablesController.CreateChain("filter", "test-chain")).To(Succeed())
			Expect(iptablesController.BulkPrependRules("test-chain", []iptables.Rule{
				fakeTCPRule,
				fakeUDPRule,
			})).To(Succeed())

			cmd := wrapCmdInNs(netnsName, exec.Command("iptables", "-w", "-S", "test-chain"))
			Expect(runForStdout(cmd)).To(ContainSubstring("-A test-chain -p udp\n-A test-chain -p tcp"))
		})

		It("returns an error when the chain does not exist", func() {
			fakeRule := new(fakes.FakeRule)
			fakeRule.FlagsReturns([]string{"--protocol", "tcp"})

			Expect(iptablesController.BulkPrependRules("test-chain", []iptables.Rule{fakeRule})).NotTo(Succeed())
		})

		Context("when there are no rules passed", func() {
			It("does nothing", func() {
				Expect(iptablesController.BulkPrependRules("test-chain", []iptables.Rule{})).To(Succeed())
				Expect(fakeRunner.ExecutedCommands()).To(BeZero())
			})
		})
	})

	Describe("DeleteChain", func() {
		BeforeEach(func() {
			Expect(iptablesController.CreateChain("filter", "test-chain")).To(Succeed())
		})

		It("deletes the chain", func() {
			Expect(iptablesController.DeleteChain("filter", "test-chain")).To(Succeed())
			exitCode, _ := run(wrapCmdInNs(netnsName, exec.Command("iptables", "-w", "-n", "-L", "test-chain")))
			Expect(exitCode).To(Equal(1))
		})

		Context("when the table is nat", func() {
			BeforeEach(func() {
				Expect(iptablesController.CreateChain("nat", "test-chain")).To(Succeed())
			})

			It("deletes the nat chain", func() {
				Expect(iptablesController.DeleteChain("nat", "test-chain")).To(Succeed())

				cmd := wrapCmdInNs(netnsName, exec.Command("iptables", "-w", "-t", "nat", "-n", "-L", "test-chain"))
				exitCode, _ := run(cmd)
				Expect(exitCode).To(Equal(1))
			})
		})

		Context("when the chain does not exist", func() {
			It("does not return an error", func() {
				Expect(iptablesController.DeleteChain("filter", "test-non-existing-chain")).To(Succeed())
			})
		})
	})

	Describe("FlushChain", func() {
		var table string

		BeforeEach(func() {
			table = "filter"
		})

		JustBeforeEach(func() {
			Expect(iptablesController.CreateChain(table, "test-chain")).To(Succeed())
			runForStdout(wrapCmdInNs(netnsName, exec.Command("iptables", "-w", "-t", table, "-A", "test-chain", "-j", "ACCEPT")))
		})

		It("flushes the chain", func() {
			Expect(iptablesController.FlushChain(table, "test-chain")).To(Succeed())

			stdout := runForStdout(wrapCmdInNs(netnsName, exec.Command("iptables", "-w", "-t", table, "-S", "test-chain")))
			Expect(stdout).NotTo(ContainSubstring("-A test-chain"))
		})

		Context("when the table is nat", func() {
			BeforeEach(func() {
				table = "nat"
			})

			It("flushes the nat chain", func() {
				Expect(iptablesController.FlushChain(table, "test-chain")).To(Succeed())

				cmd := wrapCmdInNs(netnsName, exec.Command("iptables", "-w", "-t", table, "-S", "test-chain"))
				Expect(runForStdout(cmd)).NotTo(ContainSubstring("-A test-chain"))
			})
		})

		Context("when the chain does not exist", func() {
			It("does not return an error", func() {
				Expect(iptablesController.FlushChain("filter", "test-non-existing-chain")).To(Succeed())
			})
		})
	})

	Describe("DeleteChainReferences", func() {
		var table string

		BeforeEach(func() {
			table = "filter"
		})

		JustBeforeEach(func() {
			Expect(iptablesController.CreateChain(table, "test-chain-1")).To(Succeed())
			Expect(iptablesController.CreateChain(table, "test-chain-2")).To(Succeed())

			cmd := wrapCmdInNs(netnsName, exec.Command("iptables", "-w", "-t", table, "-A", "test-chain-1", "-j", "test-chain-2"))
			runForStdout(cmd)
		})

		It("deletes the references", func() {
			Expect(iptablesController.DeleteChainReferences(table, "test-chain-1", "test-chain-2")).To(Succeed())
			cmd := wrapCmdInNs(netnsName, exec.Command("iptables", "-w", "-t", table, "-S", "test-chain-1"))
			Expect(runForStdout(cmd)).NotTo(ContainSubstring("test-chain-2"))
		})
	})

	Describe("Locking Behaviour", func() {
		Context("when something is holding the lock", func() {
			var fakeUnlocker locksmith.Unlocker
			BeforeEach(func() {
				var err error
				fakeUnlocker, err = fakeLocksmith.Lock("/foo/bar")
				Expect(err).NotTo(HaveOccurred())
			})

			It("blocks on any iptables operations until the lock is freed", func() {
				done := make(chan struct{})
				go func() {
					defer GinkgoRecover()
					Expect(iptablesController.CreateChain("filter", "test-chain")).To(Succeed())
					close(done)
				}()

				Consistently(done).ShouldNot(BeClosed())
				fakeUnlocker.Unlock()
				Eventually(done, time.Second*10).Should(BeClosed())
			})
		})

		It("should unlock, ensuring future commands can get the lock", func() {
			Expect(iptablesController.CreateChain("filter", "test-chain-1")).To(Succeed())
			Expect(iptablesController.CreateChain("filter", "test-chain-2")).To(Succeed())
		})

		It("should lock to correct key", func() {
			Expect(iptablesController.CreateChain("filter", "test-chain-1")).To(Succeed())
			Expect(fakeLocksmith.KeyForLastLock()).To(Equal(iptables.LockKey))
		})

		Context("when locking fails", func() {
			BeforeEach(func() {
				fakeLocksmith.LockReturns(nil, errors.New("failed to lock"))
			})

			It("returns the error", func() {
				Expect(iptablesController.CreateChain("filter", "test-chain")).To(MatchError("failed to lock"))
			})
		})

		Context("when running an iptables command fails", func() {
			It("still unlocks", func() {
				// this is going to fail, because the chain does not exist
				Expect(iptablesController.PrependRule("non-existent-chain", iptables.SingleFilterRule{})).NotTo(Succeed())
				Expect(iptablesController.CreateChain("filter", "test-chain-2")).To(Succeed())
			})
		})

		Context("when running an iptables command panics", func() {
			It("still unlocks", func() {
				Expect(func() { iptablesController.PrependRule("panic", iptables.SingleFilterRule{}) }).To(Panic())
				Expect(iptablesController.CreateChain("filter", "test-chain-2")).To(Succeed())
			})
		})

		Context("when unlocking fails", func() {
			BeforeEach(func() {
				fakeLocksmith.UnlockReturns(errors.New("failed to unlock"))
			})

			It("returns the error", func() {
				Expect(iptablesController.CreateChain("filter", "test-chain")).To(MatchError("failed to unlock"))
			})
		})
	})
})

func makeNamespace(nsName string) {
	runForStdout(exec.Command("ip", "netns", "add", nsName))
}

func deleteNamespace(nsName string) {
	runForStdout(exec.Command("ip", "netns", "delete", nsName))

	// We suspect that sometimes `ip netns delete` exits with 0 despite not
	// actually removing the namespace file, so we explicitly check that the
	// netns is cleaned up.
	stdout := runForStdout(exec.Command("ip", "netns"))
	Expect(stdout).NotTo(ContainSubstring(nsName))
}

func wrapCmdInNs(nsName string, cmd *exec.Cmd) *exec.Cmd {
	wrappedCmd := exec.Command("strace", "-ttT", "ip", "netns", "exec", nsName)
	wrappedCmd.Args = append(wrappedCmd.Args, cmd.Args...)
	wrappedCmd.Stdin = cmd.Stdin
	wrappedCmd.Stdout = io.MultiWriter(cmd.Stdout, GinkgoWriter)
	wrappedCmd.Stderr = io.MultiWriter(cmd.Stderr, GinkgoWriter)
	return wrappedCmd
}

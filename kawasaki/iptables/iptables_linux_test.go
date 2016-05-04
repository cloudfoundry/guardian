package iptables_test

import (
	"fmt"
	"os/exec"

	"github.com/cloudfoundry-incubator/guardian/kawasaki/iptables"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Setup", func() {
	var (
		netnsName   string
		prefix      string
		iptablesMgr *iptables.IPTables
	)

	BeforeEach(func() {
		netnsName = fmt.Sprintf("ginkgo-netns-%d", GinkgoParallelNode())
		makeNamespace(netnsName)

		fakeRunner := fake_command_runner.New()
		fakeRunner.WhenRunning(fake_command_runner.CommandSpec{},
			func(cmd *exec.Cmd) error {
				return wrapCmdInNs(netnsName, cmd).Run()
			},
		)

		prefix = fmt.Sprintf("g-%d", GinkgoParallelNode())
		iptablesMgr = iptables.New(fakeRunner, prefix)
	})

	AfterEach(func() {
		deleteNamespace(netnsName)
	})

	Describe("CreateChain", func() {
		It("creates the chain", func() {
			Expect(iptablesMgr.CreateChain("filter", "test-chain")).To(Succeed())

			sess, err := gexec.Start(wrapCmdInNs(netnsName, exec.Command("iptables", "-L", "test-chain")), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
		})

		Context("when the table is nat", func() {
			It("creates the nat chain", func() {
				Expect(iptablesMgr.CreateChain("nat", "test-chain")).To(Succeed())

				sess, err := gexec.Start(wrapCmdInNs(netnsName, exec.Command("iptables", "-t", "nat", "-L", "test-chain")), GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))
			})
		})

		Context("when the chain exists", func() {
			BeforeEach(func() {
				Expect(iptablesMgr.CreateChain("nat", "test-chain")).To(Succeed())
			})

			It("returns an error", func() {
				Expect(iptablesMgr.CreateChain("nat", "test-chain")).NotTo(Succeed())
			})
		})
	})

	Describe("DeleteChain", func() {
		BeforeEach(func() {
			Expect(iptablesMgr.CreateChain("filter", "test-chain")).To(Succeed())
		})

		It("deletes the chain", func() {
			Expect(iptablesMgr.DeleteChain("filter", "test-chain")).To(Succeed())

			sess, err := gexec.Start(wrapCmdInNs(netnsName, exec.Command("iptables", "-L", "test-chain")), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
		})

		Context("when the table is nat", func() {
			BeforeEach(func() {
				Expect(iptablesMgr.CreateChain("nat", "test-chain")).To(Succeed())
			})

			It("deletes the nat chain", func() {
				Expect(iptablesMgr.DeleteChain("nat", "test-chain")).To(Succeed())

				sess, err := gexec.Start(wrapCmdInNs(netnsName, exec.Command("iptables", "-t", "nat", "-L", "test-chain")), GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(1))
			})
		})

		Context("when the chain does not exist", func() {
			It("does not return an error", func() {
				Expect(iptablesMgr.DeleteChain("filter", "test-non-existing-chain")).To(Succeed())
			})
		})
	})

	Describe("FlushChain", func() {
		var table string

		BeforeEach(func() {
			table = "filter"
		})

		JustBeforeEach(func() {
			Expect(iptablesMgr.CreateChain(table, "test-chain")).To(Succeed())

			sess, err := gexec.Start(wrapCmdInNs(netnsName, exec.Command("iptables", "-t", table, "-A", "test-chain", "-j", "ACCEPT")), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
		})

		It("flushes the chain", func() {
			Expect(iptablesMgr.FlushChain(table, "test-chain")).To(Succeed())

			buff := gbytes.NewBuffer()
			sess, err := gexec.Start(wrapCmdInNs(netnsName, exec.Command("iptables", "-t", table, "-S", "test-chain")), buff, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
			Consistently(buff).ShouldNot(gbytes.Say("-A test-chain"))
		})

		Context("when the table is nat", func() {
			BeforeEach(func() {
				table = "nat"
			})

			It("flushes the nat chain", func() {
				Expect(iptablesMgr.FlushChain(table, "test-chain")).To(Succeed())

				buff := gbytes.NewBuffer()
				sess, err := gexec.Start(wrapCmdInNs(netnsName, exec.Command("iptables", "-t", table, "-S", "test-chain")), buff, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))
				Consistently(buff).ShouldNot(gbytes.Say("-A test-chain"))
			})
		})

		Context("when the chain does not exist", func() {
			It("does not return an error", func() {
				Expect(iptablesMgr.FlushChain("filter", "test-non-existing-chain")).To(Succeed())
			})
		})
	})

	Describe("DeleteChainReferences", func() {
		var table string

		BeforeEach(func() {
			table = "filter"
		})

		JustBeforeEach(func() {
			Expect(iptablesMgr.CreateChain(table, "test-chain-1")).To(Succeed())
			Expect(iptablesMgr.CreateChain(table, "test-chain-2")).To(Succeed())

			sess, err := gexec.Start(wrapCmdInNs(netnsName, exec.Command("iptables", "-t", table, "-A", "test-chain-1", "-j", "test-chain-2")), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
		})

		It("deletes the references", func() {
			Expect(iptablesMgr.DeleteChainReferences(table, "test-chain-1", "test-chain-2")).To(Succeed())

			Eventually(func() string {
				buff := gbytes.NewBuffer()
				sess, err := gexec.Start(wrapCmdInNs(netnsName, exec.Command("iptables", "-t", table, "-S", "test-chain-1")), buff, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))

				return string(buff.Contents())
			}).ShouldNot(ContainSubstring("test-chain-2"))
		})
	})
})

func makeNamespace(nsName string) {
	sess, err := gexec.Start(exec.Command("ip", "netns", "add", nsName), GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))
}

func deleteNamespace(nsName string) {
	sess, err := gexec.Start(exec.Command("ip", "netns", "delete", nsName), GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))
}

func wrapCmdInNs(nsName string, cmd *exec.Cmd) *exec.Cmd {
	wrappedCmd := exec.Command("ip", "netns", "exec", nsName)
	wrappedCmd.Args = append(wrappedCmd.Args, cmd.Args...)
	wrappedCmd.Stdout = cmd.Stdout
	wrappedCmd.Stderr = cmd.Stderr
	return wrappedCmd
}

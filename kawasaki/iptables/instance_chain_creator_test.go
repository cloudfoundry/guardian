package iptables_test

import (
	"errors"
	"fmt"
	"net"
	"os/exec"

	"code.cloudfoundry.org/guardian/kawasaki/iptables"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"

	. "github.com/cloudfoundry/gunk/command_runner/fake_command_runner/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Create", func() {
	var (
		fakeRunner *fake_command_runner.FakeCommandRunner
		creator    *iptables.InstanceChainCreator
		bridgeName string
		ip         net.IP
		network    *net.IPNet
		logger     lager.Logger
		handle     string
	)

	BeforeEach(func() {
		var err error

		fakeRunner = fake_command_runner.New()
		logger = lagertest.NewTestLogger("test")

		handle = "some-handle-that-is-longer-than-29-characters-long"
		bridgeName = "some-bridge"
		ip, network, err = net.ParseCIDR("1.2.3.4/28")
		Expect(err).NotTo(HaveOccurred())

		creator = iptables.NewInstanceChainCreator(
			iptables.New("/sbin/iptables", fakeRunner, "prefix-"),
		)
	})

	Describe("Container Creation", func() {
		var specs []fake_command_runner.CommandSpec

		BeforeEach(func() {
			specs = []fake_command_runner.CommandSpec{
				{
					Path: "/sbin/iptables",
					Args: []string{"--wait", "--table", "nat", "-N", "prefix-instance-some-id"},
				},
				{
					Path: "iptables",
					Args: []string{"--wait", "--table", "nat", "-A", "prefix-prerouting",
						"--jump", "prefix-instance-some-id"},
				},
				{
					Path: "sh",
					Args: []string{"-c", fmt.Sprintf(
						`(/sbin/iptables --wait --table nat -S %s | grep "\-j MASQUERADE\b" | grep -q -F -- "-s %s") || /sbin/iptables --wait --table nat -A %s --source %s ! --destination %s --jump MASQUERADE`,
						"prefix-postrouting", network.String(), "prefix-postrouting",
						network.String(), network.String(),
					)},
				},
				{
					Path: "/sbin/iptables",
					Args: []string{"--wait", "--table", "filter", "-N", "prefix-instance-some-id"},
				},
				{
					Path: "/sbin/iptables",
					Args: []string{"--wait", "-A", "prefix-instance-some-id",
						"-s", network.String(), "-d", network.String(), "-j", "ACCEPT"},
				},
				{
					Path: "/sbin/iptables",
					Args: []string{"--wait", "-A", "prefix-instance-some-id",
						"--goto", "prefix-default"},
				},
				{
					Path: "/sbin/iptables",
					Args: []string{"--wait", "-I", "prefix-forward", "2", "--in-interface", bridgeName,
						"--source", ip.String(), "--goto", "prefix-instance-some-id"},
				},
				{
					Path: "/sbin/iptables",
					Args: []string{"--wait", "--table", "filter", "-N", "prefix-instance-some-id-log"},
				},
				{
					Path: "/sbin/iptables",
					Args: []string{"--wait", "-A", "prefix-instance-some-id-log", "-m", "conntrack", "--ctstate", "NEW,UNTRACKED,INVALID",
						"--protocol", "tcp", "--jump", "LOG", "--log-prefix", "some-handle-that-is-longer-th"},
				},
				{
					Path: "/sbin/iptables",
					Args: []string{"--wait", "-A", "prefix-instance-some-id-log", "--jump", "RETURN"},
				},
			}
		})

		It("should set up the chain", func() {
			Expect(creator.Create(logger, handle, "some-id", bridgeName, ip, network)).To(Succeed())
			Expect(fakeRunner).To(HaveExecutedSerially(specs...))
		})

		DescribeTable("iptables failures",
			func(specIndex int, errorString string) {
				fakeRunner.WhenRunning(specs[specIndex], func(cmd *exec.Cmd) error {
					cmd.Stderr.Write([]byte("iptables failed"))
					return errors.New("Exit status blah")
				})

				Expect(creator.Create(logger, handle, "some-id", bridgeName, ip, network)).To(MatchError(errorString))
			},
			Entry("create nat instance chain", 0, "/sbin/iptables create-instance-chains: iptables failed"),
			Entry("bind nat instance chain to nat prerouting chain", 1, "/sbin/iptables create-instance-chains: iptables failed"),
			Entry("enable NAT for traffic coming from containers", 2, "/sbin/iptables create-instance-chains: iptables failed"),
			Entry("create logging instance chain", 7, "/sbin/iptables create-instance-chains: iptables failed"),
			Entry("append logging to instance chain", 8, "/sbin/iptables create-instance-chains: iptables failed"),
			Entry("return from logging instance chain", 9, "/sbin/iptables create-instance-chains: iptables failed"),
		)
	})

	Describe("ContainerTeardown", func() {
		var specs []fake_command_runner.CommandSpec

		BeforeEach(func() {
			specs = []fake_command_runner.CommandSpec{
				{
					Path: "sh",
					Args: []string{"-c", fmt.Sprintf(
						`/sbin/iptables --wait --table nat -S %s 2> /dev/null | grep "\-j %s\b" | sed -e "s/-A/-D/" | xargs --no-run-if-empty --max-lines=1 /sbin/iptables --wait --table nat`,
						"prefix-prerouting", "prefix-instance-some-id",
					)},
				},
				{
					Path: "sh",
					Args: []string{"-c", fmt.Sprintf(
						`/sbin/iptables --wait --table nat -F %s 2> /dev/null || true`,
						"prefix-instance-some-id",
					)},
				},
				{
					Path: "sh",
					Args: []string{"-c", fmt.Sprintf(
						`/sbin/iptables --wait --table nat -X %s 2> /dev/null || true`,
						"prefix-instance-some-id",
					)},
				},
				{
					Path: "sh",
					Args: []string{"-c", fmt.Sprintf(
						`/sbin/iptables --wait -S %s 2> /dev/null | grep "\-g %s\b" | sed -e "s/-A/-D/" | xargs --no-run-if-empty --max-lines=1 /sbin/iptables --wait`,
						"prefix-forward", "prefix-instance-some-id",
					)},
				},
				{
					Path: "sh",
					Args: []string{"-c", fmt.Sprintf("/sbin/iptables --wait --table filter -F %s 2> /dev/null || true", "prefix-instance-some-id")},
				},
				{
					Path: "sh",
					Args: []string{"-c", fmt.Sprintf("/sbin/iptables --wait --table filter -X %s 2> /dev/null || true", "prefix-instance-some-id")},
				},
				{
					Path: "sh",
					Args: []string{"-c", fmt.Sprintf("/sbin/iptables --wait --table filter -F %s 2> /dev/null || true", "prefix-instance-some-id-log")},
				},
				{
					Path: "sh",
					Args: []string{"-c", fmt.Sprintf("/sbin/iptables --wait --table filter -X %s 2> /dev/null || true", "prefix-instance-some-id-log")},
				},
			}
		})

		It("should tear down the chain", func() {
			Expect(creator.Destroy(logger, "some-id")).To(Succeed())
			Expect(fakeRunner).To(HaveExecutedSerially(specs...))
		})

		Describe("iptables failure", func() {
			It("returns an error", func() {
				fakeRunner.WhenRunning(specs[0], func(cmd *exec.Cmd) error {
					cmd.Stderr.Write([]byte("iptables failed"))
					return errors.New("exit status foo")
				})

				Expect(creator.Destroy(logger, "some-id")).To(MatchError("/sbin/iptables prune-prerouting-chain: iptables failed"))
			})
		})
	})
})

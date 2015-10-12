package iptables_test

import (
	"errors"
	"fmt"
	"net"
	"os/exec"

	"github.com/cloudfoundry-incubator/guardian/kawasaki/iptables"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/cloudfoundry/gunk/command_runner/fake_command_runner/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("filterChain", func() {
	var (
		fakeRunner *fake_command_runner.FakeCommandRunner
		testCfg    iptables.FilterConfig
		chain      iptables.Chain
		bridgeName string
		ip         net.IP
		network    *net.IPNet
	)

	BeforeEach(func() {
		var err error

		fakeRunner = fake_command_runner.New()
		testCfg = iptables.FilterConfig{
			InputChain:   "filter-input-chain",
			ForwardChain: "filter-forward-chain",
			DefaultChain: "filter-default-chain",
		}

		bridgeName = "some-bridge"
		ip, network, err = net.ParseCIDR("1.2.3.4/28")
		Expect(err).NotTo(HaveOccurred())

		chain = iptables.NewFilterChain(testCfg, fakeRunner, lagertest.NewTestLogger("test"))
	})

	Describe("Setup", func() {
		var specs []fake_command_runner.CommandSpec

		BeforeEach(func() {
			specs = []fake_command_runner.CommandSpec{
				fake_command_runner.CommandSpec{
					Path: "iptables",
					Args: []string{"--wait", "-N", "some-chain"},
				},
				fake_command_runner.CommandSpec{
					Path: "iptables",
					Args: []string{"--wait", "-A", "some-chain",
						"-s", network.String(), "-d", network.String(), "-j", "ACCEPT"},
				},
				fake_command_runner.CommandSpec{
					Path: "iptables",
					Args: []string{"--wait", "-A", "some-chain",
						"--goto", testCfg.DefaultChain},
				},
				fake_command_runner.CommandSpec{
					Path: "iptables",
					Args: []string{"--wait", "-I", testCfg.ForwardChain, "2", "--in-interface", bridgeName,
						"--source", ip.String(), "--goto", "some-chain"},
				},
			}
		})

		It("should set up the chain", func() {
			Expect(chain.Setup("some-chain", bridgeName, ip, network)).To(Succeed())

			Expect(fakeRunner).To(HaveExecutedSerially(specs...))
		})

		DescribeTable("iptables failures",
			func(specIndex int, errorString string) {
				fakeRunner.WhenRunning(specs[specIndex], func(*exec.Cmd) error {
					return errors.New("iptables failed")
				})
				Expect(chain.Setup("some-chain", bridgeName, ip, network)).To(MatchError(errorString))

			},
			Entry("create filter instance chain", 0, "iptables_manager: filter: iptables failed"),
			Entry("allow intra-subnet traffic", 1, "iptables_manager: filter: iptables failed"),
			Entry("use the default filter chain otherwise", 2, "iptables_manager: filter: iptables failed"),
			Entry("bind filter instance chain to filter forward chain", 3, "iptables_manager: filter: iptables failed"),
		)
	})

	Describe("Teardown", func() {
		var specs []fake_command_runner.CommandSpec

		BeforeEach(func() {
			specs = []fake_command_runner.CommandSpec{
				fake_command_runner.CommandSpec{
					Path: "sh",
					Args: []string{"-c", fmt.Sprintf(
						`iptables --wait -S %s 2> /dev/null | grep "\-g %s\b" | sed -e "s/-A/-D/" | xargs --no-run-if-empty --max-lines=1 iptables --wait`,
						testCfg.ForwardChain, "some-chain",
					)},
				},
				fake_command_runner.CommandSpec{
					Path: "sh",
					Args: []string{"-c", fmt.Sprintf("iptables --wait -F %s 2> /dev/null || true", "some-chain")},
				},
				fake_command_runner.CommandSpec{
					Path: "sh",
					Args: []string{"-c", fmt.Sprintf("iptables --wait -X %s 2> /dev/null || true", "some-chain")},
				},
			}
		})

		It("should tear down the chain", func() {
			Expect(chain.Teardown("some-chain")).To(Succeed())

			Expect(fakeRunner).To(HaveExecutedSerially(specs...))
		})

		DescribeTable("iptables failures",
			func(specIndex int, errorString string) {
				fakeRunner.WhenRunning(specs[specIndex], func(*exec.Cmd) error {
					return errors.New("iptables failed")
				})

				Expect(chain.Teardown("some-chain")).To(MatchError(errorString))
			},
			Entry("prune forward chain", 0, "iptables_manager: filter: iptables failed"),
			Entry("flush instance chain", 1, "iptables_manager: filter: iptables failed"),
			Entry("delete instance chain", 2, "iptables_manager: filter: iptables failed"),
		)
	})
})

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

var _ = Describe("natChain", func() {
	var (
		fakeRunner *fake_command_runner.FakeCommandRunner
		testCfg    iptables.NATConfig
		chain      iptables.Chain
		bridgeName string
		ip         net.IP
		network    *net.IPNet
	)

	BeforeEach(func() {
		var err error

		fakeRunner = fake_command_runner.New()
		testCfg = iptables.NATConfig{
			PreroutingChain:  "nat-prerouting-chain",
			PostroutingChain: "nat-postrouting-chain",
		}

		bridgeName = "some-bridge"
		ip, network, err = net.ParseCIDR("1.2.3.4/28")
		Expect(err).NotTo(HaveOccurred())

		chain = iptables.NewNATChain(testCfg, fakeRunner, lagertest.NewTestLogger("test"))
	})

	Describe("ContainerSetup", func() {
		var specs []fake_command_runner.CommandSpec
		BeforeEach(func() {
			specs = []fake_command_runner.CommandSpec{
				fake_command_runner.CommandSpec{
					Path: "iptables",
					Args: []string{"--wait", "--table", "nat", "-N", "some-chain"},
				},
				fake_command_runner.CommandSpec{
					Path: "iptables",
					Args: []string{"--wait", "--table", "nat", "-A", testCfg.PreroutingChain,
						"--jump", "some-chain"},
				},
				fake_command_runner.CommandSpec{
					Path: "sh",
					Args: []string{"-c", fmt.Sprintf(
						`(iptables --wait --table nat -S %s | grep "\-j MASQUERADE\b" | grep -q -F -- "-s %s") || iptables --wait --table nat -A %s --source %s ! --destination %s --jump MASQUERADE`,
						testCfg.PostroutingChain, network.String(), testCfg.PostroutingChain,
						network.String(), network.String(),
					)},
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
			Entry("create nat instance chain", 0, "iptables_manager: nat: iptables failed"),
			Entry("bind nat instance chain to nat prerouting chain", 1, "iptables_manager: nat: iptables failed"),
			Entry("enable NAT for traffic coming from containers", 2, "iptables_manager: nat: iptables failed"),
		)
	})

	Describe("ContainerTeardown", func() {
		var specs []fake_command_runner.CommandSpec

		Describe("nat chain", func() {
			BeforeEach(func() {
				specs = []fake_command_runner.CommandSpec{
					fake_command_runner.CommandSpec{
						Path: "sh",
						Args: []string{"-c", fmt.Sprintf(
							`iptables --wait --table nat -S %s 2> /dev/null | grep "\-j %s\b" | sed -e "s/-A/-D/" | xargs --no-run-if-empty --max-lines=1 iptables --wait --table nat`,
							testCfg.PreroutingChain, "some-chain",
						)},
					},
					fake_command_runner.CommandSpec{
						Path: "sh",
						Args: []string{"-c", fmt.Sprintf(
							`iptables --wait --table nat -F %s 2> /dev/null || true`,
							"some-chain",
						)},
					},
					fake_command_runner.CommandSpec{
						Path: "sh",
						Args: []string{"-c", fmt.Sprintf(
							`iptables --wait --table nat -X %s 2> /dev/null || true`,
							"some-chain",
						)},
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
				Entry("prune prerouting chain", 0, "iptables_manager: nat: iptables failed"),
				Entry("flush instance chain", 1, "iptables_manager: nat: iptables failed"),
				Entry("delete instance chain", 2, "iptables_manager: nat: iptables failed"),
			)
		})
	})
})

package netplugin_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net"
	"os/exec"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/kawasaki"
	"code.cloudfoundry.org/guardian/kawasaki/kawasakifakes"
	"code.cloudfoundry.org/guardian/netplugin"
	"code.cloudfoundry.org/guardian/properties"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

func mustMarshalJSON(input interface{}) string {
	bytes, err := json.Marshal(input)
	Expect(err).NotTo(HaveOccurred())
	return string(bytes)
}

var _ = Describe("ExternalNetworker", func() {
	var (
		containerSpec        garden.ContainerSpec
		configStore          kawasaki.ConfigStore
		fakeCommandRunner    *fake_command_runner.FakeCommandRunner
		logger               *lagertest.TestLogger
		plugin               netplugin.ExternalNetworker
		handle               string
		resolvConfigurer     *kawasakifakes.FakeDnsResolvConfigurer
		pluginOutput         string
		pluginErr            error
		dnsServers           = []net.IP{net.ParseIP("8.8.8.8"), net.ParseIP("9.9.9.9")}
		additionalDNSServers = []net.IP{net.ParseIP("11.11.11.11")}
	)

	BeforeEach(func() {
		inputProperties := garden.Properties{
			"some-key":               "some-value",
			"some-other-key":         "some-other-value",
			"network.some-key":       "some-network-value",
			"network.some-other-key": "some-other-network-value",
		}
		fakeCommandRunner = fake_command_runner.New()
		configStore = properties.NewManager()
		handle = "some-handle"
		containerSpec = garden.ContainerSpec{
			Handle:     "some-handle",
			Network:    "potato",
			Properties: inputProperties,
		}
		logger = lagertest.NewTestLogger("test")
		externalIP := net.ParseIP("1.2.3.4")
		resolvConfigurer = &kawasakifakes.FakeDnsResolvConfigurer{}
		plugin = netplugin.New(
			fakeCommandRunner,
			configStore,
			externalIP,
			dnsServers,
			additionalDNSServers,
			resolvConfigurer,
			"some/path",
			[]string{"arg1", "arg2", "arg3"},
		)

		pluginErr = nil
		fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
			Path: "some/path",
		}, func(cmd *exec.Cmd) error {
			cmd.Stdout.Write([]byte(pluginOutput))
			cmd.Stderr.Write([]byte("some-stderr-bytes"))
			return pluginErr
		})
	})

	Describe("Network", func() {
		It("passes the pid of the container to the external plugin's stdin", func() {
			err := plugin.Network(logger, containerSpec, 42)
			Expect(err).NotTo(HaveOccurred())

			cmd := fakeCommandRunner.ExecutedCommands()[0]
			input, err := ioutil.ReadAll(cmd.Stdin)
			Expect(err).NotTo(HaveOccurred())

			Expect(string(input)).To(ContainSubstring("42"))
		})

		It("executes the external plugin with the correct args and input", func() {
			err := plugin.Network(logger, containerSpec, 42)
			Expect(err).NotTo(HaveOccurred())

			cmd := fakeCommandRunner.ExecutedCommands()[0]
			Expect(cmd.Path).To(Equal("some/path"))

			Expect(cmd.Args).To(Equal([]string{
				"some/path",
				"arg1",
				"arg2",
				"arg3",
				"--action", "up",
				"--handle", "some-handle",
			}))

			pluginInput, err := ioutil.ReadAll(cmd.Stdin)
			Expect(err).NotTo(HaveOccurred())
			Expect(pluginInput).To(MatchJSON(`{
				"Pid": 42,
				"Properties": {
					"some-key": "some-network-value",
					"some-other-key": "some-other-network-value"
				}
			}`))
		})

		Context("when there are NetOut rules provided", func() {
			BeforeEach(func() {
				containerSpec.NetOut = []garden.NetOutRule{
					garden.NetOutRule{
						Protocol: garden.ProtocolTCP,
						Networks: []garden.IPRange{garden.IPRangeFromIP(net.ParseIP("8.8.8.8"))},
						Ports:    []garden.PortRange{garden.PortRangeFromPort(53)},
					},
					garden.NetOutRule{
						Protocol: garden.ProtocolTCP,
						Networks: []garden.IPRange{garden.IPRangeFromIP(net.ParseIP("8.8.4.4"))},
						Ports:    []garden.PortRange{garden.PortRangeFromPort(53)},
					},
				}
			})

			It("passes them in the stdin to the network plugin", func() {
				Expect(plugin.Network(logger, containerSpec, 42)).To(Succeed())

				cmd := fakeCommandRunner.ExecutedCommands()[0]
				pluginInput, err := ioutil.ReadAll(cmd.Stdin)
				Expect(err).NotTo(HaveOccurred())
				Expect(pluginInput).To(MatchJSON(`{
				"Pid": 42,
				"Properties": {
					"some-key": "some-network-value",
					"some-other-key": "some-other-network-value"
				},
				"netout_rules": [
				  {
					  "protocol":1,
					  "networks": [{"start":"8.8.8.8","end":"8.8.8.8"}],
					  "ports": [{"start":53,"end":53}]
					},
					{
					  "protocol":1,
					  "networks":[{"start":"8.8.4.4","end":"8.8.4.4"}],
					  "ports":[{"start":53,"end":53}]
					}
				]
			}`))
			})
		})

		Context("when NetIn input is provided", func() {
			BeforeEach(func() {
				containerSpec.NetIn = []garden.NetIn{
					garden.NetIn{
						HostPort:      9999,
						ContainerPort: 8080,
					},
					garden.NetIn{
						HostPort:      9989,
						ContainerPort: 8081,
					},
				}
			})

			It("passes the input through stdin to the network plugin", func() {
				Expect(plugin.Network(logger, containerSpec, 42)).To(Succeed())

				cmd := fakeCommandRunner.ExecutedCommands()[0]
				pluginInput, err := ioutil.ReadAll(cmd.Stdin)
				Expect(err).NotTo(HaveOccurred())

				Expect(pluginInput).To(MatchJSON(`{
				"Pid": 42,
				"Properties": {
					"some-key": "some-network-value",
					"some-other-key": "some-other-network-value"
				},
				"netin": [
					{
						"host_port": 9999,
						"container_port": 8080
					},
					{
						"host_port": 9989,
						"container_port": 8081
					}
				]
			}`))
			})
		})

		It("collects and logs the stderr from the plugin", func() {
			err := plugin.Network(logger, containerSpec, 42)
			Expect(err).NotTo(HaveOccurred())

			Expect(logger).To(gbytes.Say("result.*some-stderr-bytes"))
		})

		Describe("DNS configuration inside the container", func() {
			var cfg kawasaki.NetworkConfig

			JustBeforeEach(func() {
				var (
					log lager.Logger
					pid int
				)

				err := plugin.Network(logger, containerSpec, 42)
				Expect(err).NotTo(HaveOccurred())

				Expect(resolvConfigurer.ConfigureCallCount()).To(Equal(1))
				log, cfg, pid = resolvConfigurer.ConfigureArgsForCall(0)
				Expect(log).To(Equal(logger))
				Expect(pid).To(Equal(42))
			})

			Context("when the external plugin returns a containerIP in properties", func() {
				BeforeEach(func() {
					pluginOutput = `{
						"properties": {
							"garden.network.container-ip": "10.255.1.2"
						}
				  }`
				})

				It("is configured according to Garden defaults", func() {
					Expect(cfg).To(Equal(kawasaki.NetworkConfig{
						ContainerIP:           net.ParseIP("10.255.1.2"),
						BridgeIP:              net.ParseIP("10.255.1.2"),
						ContainerHandle:       "some-handle",
						PluginNameservers:     nil,
						OperatorNameservers:   dnsServers,
						AdditionalNameservers: additionalDNSServers,
					}))
				})
			})

			Context("when the external plugin returns a containerIP in properties and dns_servers", func() {
				Context("when 0 DNS servers are returned", func() {
					BeforeEach(func() {
						pluginOutput = `{
							"properties": {
								"garden.network.container-ip": "10.255.1.2"
							},
							"dns_servers": []
						}`
					})

					It("is configured using the returned DNS servers", func() {
						Expect(cfg).To(Equal(kawasaki.NetworkConfig{
							ContainerIP:           net.ParseIP("10.255.1.2"),
							BridgeIP:              net.ParseIP("10.255.1.2"),
							ContainerHandle:       "some-handle",
							PluginNameservers:     []net.IP{},
							OperatorNameservers:   dnsServers,
							AdditionalNameservers: additionalDNSServers,
						}))
					})
				})

				Context("when 1 DNS server is returned", func() {
					BeforeEach(func() {
						pluginOutput = `{
							"properties": {
								"garden.network.container-ip": "10.255.1.2"
							},
							"dns_servers": [
								"1.2.3.4"
							]
						}`
					})

					It("is configured using the returned DNS servers", func() {
						Expect(cfg).To(Equal(kawasaki.NetworkConfig{
							ContainerIP:           net.ParseIP("10.255.1.2"),
							BridgeIP:              net.ParseIP("10.255.1.2"),
							ContainerHandle:       "some-handle",
							PluginNameservers:     []net.IP{net.ParseIP("1.2.3.4")},
							OperatorNameservers:   dnsServers,
							AdditionalNameservers: additionalDNSServers,
						}))
					})
				})

				Context("when > 1 DNS server is returned", func() {
					BeforeEach(func() {
						pluginOutput = `{
							"properties": {
								"garden.network.container-ip": "10.255.1.2"
							},
							"dns_servers": [
								"1.2.3.4",
								"1.2.3.5"
							]
						}`
					})

					It("is configured using the returned DNS servers", func() {
						Expect(cfg).To(Equal(kawasaki.NetworkConfig{
							ContainerIP:           net.ParseIP("10.255.1.2"),
							BridgeIP:              net.ParseIP("10.255.1.2"),
							ContainerHandle:       "some-handle",
							PluginNameservers:     []net.IP{net.ParseIP("1.2.3.4"), net.ParseIP("1.2.3.5")},
							OperatorNameservers:   dnsServers,
							AdditionalNameservers: additionalDNSServers,
						}))
					})
				})
			})
		})

		Context("when the external plugin errors", func() {
			BeforeEach(func() {
				pluginErr = errors.New("external-plugin-error")
			})

			It("returns the error", func() {
				Expect(plugin.Network(logger, containerSpec, 42)).To(MatchError("external networker up: external-plugin-error"))
			})

			It("collects and logs the stderr from the plugin", func() {
				plugin.Network(logger, containerSpec, 42)
				Expect(logger).To(gbytes.Say("result.*error.*some-stderr-bytes"))
			})
		})

		Context("when the external plugin returns valid properties JSON", func() {
			It("persists the returned properties to the container's properties", func() {
				pluginOutput = `{"properties":{"foo":"bar","ping":"pong","garden.network.container-ip":"10.255.1.2"}}`

				err := plugin.Network(logger, containerSpec, 42)
				Expect(err).NotTo(HaveOccurred())

				persistedPropertyValue, _ := configStore.Get("some-handle", "foo")
				Expect(persistedPropertyValue).To(Equal("bar"))
			})
		})

		Context("when the external plugin returns invalid JSON", func() {
			It("returns a useful error message", func() {
				pluginOutput = "invalid-json"

				err := plugin.Network(logger, containerSpec, 42)
				Expect(err).To(MatchError(ContainSubstring("unmarshaling result from external networker")))
			})
		})

		Context("when the external network plugin returns nothing", func() {
			It("succeeds", func() {
				pluginOutput = ""

				err := plugin.Network(logger, containerSpec, 42)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("Destroy", func() {
		It("executes the external plugin with the correct args", func() {
			Expect(plugin.Destroy(logger, "my-handle")).To(Succeed())

			cmd := fakeCommandRunner.ExecutedCommands()[0]
			Expect(cmd.Path).To(Equal("some/path"))

			Expect(cmd.Args).To(Equal([]string{
				"some/path",
				"arg1",
				"arg2",
				"arg3",
				"--action", "down",
				"--handle", "my-handle",
			}))
		})

		Context("when the external plugin errors", func() {
			BeforeEach(func() {
				pluginErr = errors.New("boom")
			})
			It("returns the error", func() {
				Expect(plugin.Destroy(logger, "my-handle")).To(MatchError("external networker down: boom"))
			})
		})
	})

	Describe("NetIn", func() {
		BeforeEach(func() {
			configStore.Set(handle, gardener.ContainerIPKey, "5.6.7.8")
			pluginOutput = `{
					"host_port": 1234,
					"container_port": 5555
				}`
		})

		It("executes the external plugin with the correct args and stdin", func() {
			_, _, err := plugin.NetIn(logger, handle, 22, 33)
			Expect(err).NotTo(HaveOccurred())

			cmd := fakeCommandRunner.ExecutedCommands()[0]
			Expect(cmd.Path).To(Equal("some/path"))

			Expect(cmd.Args).To(Equal([]string{
				"some/path",
				"arg1",
				"arg2",
				"arg3",
				"--action", "net-in",
				"--handle", "some-handle",
			}))

			pluginInput, err := ioutil.ReadAll(cmd.Stdin)
			Expect(err).NotTo(HaveOccurred())
			Expect(pluginInput).To(MatchJSON(`{
				"HostIP": "1.2.3.4",
				"HostPort" : 22,
				"ContainerIP": "5.6.7.8",
				"ContainerPort": 33
			}`))
		})

		It("adds the port mapping output from the external plugin", func() {
			externalPort, containerPort, err := plugin.NetIn(logger, handle, 22, 33)
			Expect(err).NotTo(HaveOccurred())

			portMapping, ok := configStore.Get(handle, gardener.MappedPortsKey)
			Expect(ok).To(BeTrue())
			Expect(portMapping).To(MatchJSON(mustMarshalJSON([]garden.PortMapping{
				{
					HostPort:      1234,
					ContainerPort: 5555,
				},
			})))
			Expect(externalPort).To(Equal(uint32(1234)))
			Expect(containerPort).To(Equal(uint32(5555)))
		})

		Context("when the handle cannot be found in the store", func() {
			It("returns an error", func() {
				_, _, err := plugin.NetIn(logger, "some-nonexistent-handle", 22, 33)
				Expect(err).To(MatchError("cannot find container [some-nonexistent-handle]\n"))
			})
		})

		Context("when the external plugin errors", func() {
			BeforeEach(func() {
				pluginErr = errors.New("potato")
			})
			It("returns the error", func() {
				_, _, err := plugin.NetIn(logger, handle, 22, 33)
				Expect(err).To(MatchError("external networker net-in: potato"))
			})
		})

		Context("when adding the port mapping fails", func() {
			BeforeEach(func() {
				configStore.Set(handle, gardener.MappedPortsKey, "%%%%%%")
			})
			It("returns the error", func() {
				_, _, err := plugin.NetIn(logger, handle, 123, 543)
				Expect(err).To(MatchError(ContainSubstring("invalid character")))
			})
		})

		It("collects and logs the stderr from the plugin", func() {
			_, _, err := plugin.NetIn(logger, handle, 22, 33)
			Expect(err).NotTo(HaveOccurred())

			Expect(logger).To(gbytes.Say("result.*some-stderr-bytes"))
		})
	})

	Describe("NetOut", func() {
		var handle = "my-handle"
		var rule garden.NetOutRule

		BeforeEach(func() {
			configStore.Set(handle, gardener.ContainerIPKey, "169.254.1.2")
			rule = createRule("1.1.1.1", "2.2.2.2", 9000, 9999)
		})

		It("executes the external plugin with the correct args and input", func() {
			Expect(plugin.NetOut(logger, handle, rule)).To(Succeed())

			cmd := fakeCommandRunner.ExecutedCommands()[0]
			Expect(cmd.Path).To(Equal("some/path"))
			Expect(cmd.Args).To(Equal([]string{
				"some/path",
				"arg1",
				"arg2",
				"arg3",
				"--action", "net-out",
				"--handle", handle,
			}))

			checkPluginArgs(cmd, rule)
		})

		Context("when the handle cannot be found in the config store", func() {
			It("returns the error", func() {
				Expect(plugin.NetOut(logger, "missing-handle", rule)).To(MatchError("cannot find container [missing-handle]\n"))
			})
		})

		Context("when the external plugin errors", func() {
			BeforeEach(func() {
				pluginErr = errors.New("boom")
			})
			It("returns the error", func() {
				Expect(plugin.NetOut(logger, handle, rule)).To(MatchError("external networker net-out: boom"))
			})
		})

		It("collects and logs the stderr from the plugin", func() {
			Expect(plugin.NetOut(logger, handle, rule)).To(Succeed())

			Expect(logger).To(gbytes.Say("result.*some-stderr-bytes"))
		})
	})

	Describe("BulkNetOut", func() {
		var handle = "my-handle"
		var rules []garden.NetOutRule

		BeforeEach(func() {
			configStore.Set(handle, gardener.ContainerIPKey, "169.254.1.2")
			rules = []garden.NetOutRule{
				createRule("1.1.1.1", "2.2.2.2", 1111, 2222),
				createRule("3.3.3.3", "4.4.4.4", 3333, 4444),
			}
		})

		It("calls BulkNetOut passing all the rules", func() {
			Expect(plugin.BulkNetOut(logger, handle, rules)).To(Succeed())

			Expect(fakeCommandRunner.ExecutedCommands()).To(HaveLen(1))
			checkBulkPluginArgs(fakeCommandRunner.ExecutedCommands()[0], rules)
		})

		Context("when the handle cannot be found in the config store", func() {
			It("returns the error", func() {
				Expect(plugin.BulkNetOut(logger, "missing-handle", rules)).To(MatchError("cannot find container [missing-handle]\n"))
			})
		})

		Context("when the external plugin errors", func() {
			BeforeEach(func() {
				pluginErr = errors.New("boom")
			})

			It("returns the error", func() {
				Expect(plugin.BulkNetOut(logger, handle, rules)).To(MatchError("external networker bulk-net-out: boom"))
			})
		})

		It("collects and logs the stderr from the plugin", func() {
			Expect(plugin.BulkNetOut(logger, handle, rules)).To(Succeed())

			Expect(logger).To(gbytes.Say("result.*some-stderr-bytes"))
		})
	})
})

func createRule(netStart, netEnd string, portStart, portEnd int) garden.NetOutRule {
	return garden.NetOutRule{
		Protocol: garden.ProtocolTCP,
		Networks: []garden.IPRange{{
			Start: net.ParseIP(netStart),
			End:   net.ParseIP(netEnd),
		}},
		Ports: []garden.PortRange{{
			Start: uint16(portStart),
			End:   uint16(portEnd),
		}},
	}
}

func checkPluginArgs(cmd *exec.Cmd, rule garden.NetOutRule) {
	pluginInput, err := ioutil.ReadAll(cmd.Stdin)
	Expect(err).NotTo(HaveOccurred())
	r := &netplugin.NetOutInputs{}
	json.Unmarshal(pluginInput, r)
	Expect(r.NetOutRule).To(Equal(rule))
}

func checkBulkPluginArgs(cmd *exec.Cmd, rules []garden.NetOutRule) {
	pluginInput, err := ioutil.ReadAll(cmd.Stdin)
	Expect(err).NotTo(HaveOccurred())
	r := &netplugin.BulkNetOutInputs{}
	json.Unmarshal(pluginInput, r)
	Expect(r.NetOutRules).To(Equal(rules))
}

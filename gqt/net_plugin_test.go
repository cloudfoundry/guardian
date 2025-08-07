package gqt_test

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Network plugin", func() {
	var (
		client    *runner.RunningGarden
		container garden.Container

		containerSpec    garden.ContainerSpec
		containerNetwork string

		extraProperties garden.Properties

		argsFile        string
		stdinFile       string
		pluginReturn    string
		hostNameservers []string
		tmpDir          string
	)

	BeforeEach(func() {

		containerNetwork = fmt.Sprintf("192.168.%d.0/24", 12+GinkgoParallelProcess())
		containerSpec = garden.ContainerSpec{}

		tmpDir = tempDir("", "netplugtest")

		argsFile = path.Join(tmpDir, "args.log")
		stdinFile = path.Join(tmpDir, "stdin.log")

		config.NetworkPluginBin = binaries.NetworkPlugin
		config.NetworkPluginExtraArgs = []string{"--args-file", argsFile, "--stdin-file", stdinFile}

		pluginReturn = `{"properties":{"garden.network.container-ip":"10.255.10.10"}}`

		out := readFileString("/etc/resolv.conf")
		hostNameservers = parseEntries(out, "nameserver")
	})

	JustBeforeEach(func() {
		var err error

		config.NetworkPluginExtraArgs = append(
			config.NetworkPluginExtraArgs,
			"--output", pluginReturn,
		)
		client = runner.Start(config)

		containerSpec.Network = containerNetwork
		containerSpec.Properties = extraProperties
		container, err = client.Create(containerSpec)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	It("executes the network plugin during container creation", func() {
		containerHandle := container.Handle()

		Eventually(getContent(argsFile)).Should(
			ContainSubstring(
				fmt.Sprintf("--action up --handle %s", containerHandle),
			),
		)
	})

	It("executes the network plugin during container destroy", func() {
		containerHandle := container.Handle()

		Expect(client.Destroy(containerHandle)).To(Succeed())
		Expect(argsFile).To(BeAnExistingFile())

		Eventually(getContent(argsFile)).Should(ContainSubstring(fmt.Sprintf("--action down --handle %s", containerHandle)))
	})

	It("passes the container pid to plugin's stdin", func() {
		Eventually(getContent(stdinFile)).Should(
			MatchRegexp(`.*{"Pid":[0-9]+.*}.*`),
		)
	})

	It("sets the ExternalIP and ContainerIP fields on the container.Info()", func() {
		info, err := container.Info()
		Expect(err).NotTo(HaveOccurred())

		Expect(info.ExternalIP).NotTo(BeEmpty())
		Expect(info.ContainerIP).To(Equal("10.255.10.10"))
	})

	Context("when the containerSpec contains NetOutRules", func() {
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

		It("passes the NetOut rules to the plugin during container creation", func() {
			jsonBytes, err := json.Marshal(containerSpec.NetOut)
			Expect(err).NotTo(HaveOccurred())

			Eventually(getContent(stdinFile)).Should(
				ContainSubstring("\"netout_rules\":" + string(jsonBytes)),
			)
		})
	})

	Context("when the containerSpec contains NetIn", func() {
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

		It("passes the NetIn input to the plugin during container creation", func() {
			jsonBytes, err := json.Marshal(containerSpec.NetIn)
			Expect(err).NotTo(HaveOccurred())

			Eventually(getContent(stdinFile)).Should(
				ContainSubstring("\"netin\":" + string(jsonBytes)),
			)
		})
	})

	Context("when BulkNetOut is called", func() {
		It("passes down the bulk net out rules to the external networker", func() {
			rules := []garden.NetOutRule{
				garden.NetOutRule{
					Protocol: garden.ProtocolTCP,
				},
				garden.NetOutRule{
					Protocol: garden.ProtocolUDP,
				},
			}
			container.BulkNetOut(rules)

			Eventually(getContent(stdinFile)).Should(
				ContainSubstring(`{"container_ip":"10.255.10.10","netout_rules":[{"protocol":1},{"protocol":2}]}`),
			)
		})
	})

	It("adds the host's non-127.0.0.0/24 DNS servers to the container's /etc/resolv.conf", func() {
		resolvConf := readResolvConf(container)

		for _, hostNameserver := range hostNameservers {
			Expect(resolvConf).To(ContainSubstring(hostNameserver))
			Expect(resolvConf).NotTo(ContainSubstring("127.0.0."))
		}
	})

	It("adds the host's search domains to the container's /etc/resolv.conf", func() {
		containerSearchDomains := getSearchDomains(container)
		resolvConf := readFileString("/etc/resolv.conf")
		hostSearchDomains := parseEntries(resolvConf, "search")

		Expect(containerSearchDomains).To(ConsistOf(hostSearchDomains))
	})

	Context("when --dns-server is provided", func() {
		BeforeEach(func() {
			config.DNSServers = []string{"1.2.3.4"}
		})

		It("adds the IP address to the container's /etc/resolv.conf", func() {
			nameservers := getNameservers(container)
			Expect(nameservers).To(ContainElement("1.2.3.4"))
		})

		It("strips the host's DNS servers from the container's /etc/resolv.conf", func() {
			nameservers := getNameservers(container)

			for _, hostNameserver := range hostNameservers {
				Expect(nameservers).NotTo(ContainElement(hostNameserver))
			}
		})
	})

	Context("when --additional-dns-server is provided", func() {
		BeforeEach(func() {
			config.AdditionalDNSServers = []string{"1.2.3.4"}
		})

		It("writes the IP address and the host's non-127.0.0.0/24 DNS servers to the container's /etc/resolv.conf", func() {
			resolvConf := readResolvConf(container)

			for _, hostNameserver := range hostNameservers {
				Expect(resolvConf).To(ContainSubstring(hostNameserver))
				Expect(resolvConf).NotTo(ContainSubstring("127.0.0."))
			}

			Expect(resolvConf).To(ContainSubstring("nameserver 1.2.3.4"))
		})
	})

	Context("when --dns-server and --additional-dns-server are provided", func() {
		BeforeEach(func() {
			config.DNSServers = []string{"1.2.3.4"}
			config.AdditionalDNSServers = []string{"1.2.3.5"}
		})

		It("writes the --dns-server and --additional-dns-server DNS servers to the container's /etc/resolv.conf", func() {
			resolvConf := readResolvConf(container)
			Expect(resolvConf).To(Equal("nameserver 1.2.3.4\nnameserver 1.2.3.5\n"))
		})
	})

	Context("and the plugin is essentially a noop", func() {
		BeforeEach(func() {
			config.NetworkPluginBin = "/bin/true"
		})

		It("successfully creates a container", func() {
			_, err := client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when the container spec has properties", func() {
		BeforeEach(func() {
			extraProperties = garden.Properties{
				"some-property-on-the-spec": "some-value",
				"network.some-key":          "some-value",
				"network.some-other-key":    "some-other-value",
				"some-other-key":            "do-not-propagate",
			}
		})

		It("propagates the 'network.*' properties to the network plugin up action", func() {
			expectedJSON := `"some-key":"some-value","some-other-key":"some-other-value"}`
			Eventually(getContent(stdinFile)).Should(ContainSubstring(expectedJSON))
		})

		It("doesn't remove existing properties", func() {
			info, err := container.Info()
			Expect(err).NotTo(HaveOccurred())

			Expect(info.Properties).To(HaveKey("some-property-on-the-spec"))
		})
	})

	Context("when the network plugin returns properties", func() {
		BeforeEach(func() {
			pluginReturn = `{
					"properties":{
						"foo":"bar",
						"garden.network.container-ip":"10.255.10.10",
						"garden.network.host-ip":"255.255.255.255"
					}
			  }`
		})

		It("persists the returned properties to the container's properties", func() {
			info, err := container.Info()
			Expect(err).NotTo(HaveOccurred())

			containerProperties := info.Properties

			Expect(containerProperties["foo"]).To(Equal("bar"))
			Expect(containerProperties["garden.network.container-ip"]).To(Equal("10.255.10.10"))
			Expect(containerProperties["garden.network.host-ip"]).To(Equal("255.255.255.255"))
		})

		Context("and the properties contain ipv6 address", func() {
			BeforeEach(func() {
				pluginReturn = `{
					"properties":{
						"foo":"bar",
						"garden.network.container-ip":"10.255.10.10",
						"garden.network.container-ipv6":"2001:db8::1",
						"garden.network.host-ip":"255.255.255.255"
					}
			  }`
			})

			It("persists the returned properties to the container's properties", func() {
				info, err := container.Info()
				Expect(err).NotTo(HaveOccurred())

				containerProperties := info.Properties

				Expect(containerProperties["foo"]).To(Equal("bar"))
				Expect(containerProperties["garden.network.container-ip"]).To(Equal("10.255.10.10"))
				Expect(containerProperties["garden.network.container-ip"]).To(Equal("2001:db8::1"))
				Expect(containerProperties["garden.network.host-ip"]).To(Equal("255.255.255.255"))
			})
		})
	})

	Context("when the network plugin returns dns_servers", func() {
		BeforeEach(func() {
			pluginReturn = `{
					"properties":{
						"garden.network.container-ip":"10.255.10.10"
					},
					"dns_servers": [
						"1.2.3.4",
						"1.2.3.5"
					]
			  }`
		})

		It("sets the nameserver entries in the container's /etc/resolv.conf to the values supplied by the network plugin", func() {
			Expect(getNameservers(container)).To(Equal([]string{"1.2.3.4", "1.2.3.5"}))
		})

		Context("when the rootFS does not contain /etc/resolv.conf", func() {
			var rootFSWithoutHostsAndResolv string

			BeforeEach(func() {
				rootFSWithoutHostsAndResolv = createRootfs(func(root string) {
					Expect(os.Chmod(filepath.Join(root, "tmp"), 0777)).To(Succeed())
					Expect(os.Remove(filepath.Join(root, "etc", "hosts"))).To(Succeed())
					Expect(os.Remove(filepath.Join(root, "etc", "resolv.conf"))).To(Succeed())
				}, 0755)

				containerSpec.RootFSPath = fmt.Sprintf("raw://%s", rootFSWithoutHostsAndResolv)
			})

			AfterEach(func() {
				Expect(os.RemoveAll(filepath.Dir(rootFSWithoutHostsAndResolv))).To(Succeed())
			})

			It("sets the nameserver entries in the container's /etc/resolv.conf to the values supplied by the network plugin", func() {
				Expect(getNameservers(container)).To(Equal([]string{"1.2.3.4", "1.2.3.5"}))
			})
		})
	})

	Context("when the network plugin returns search_domains", func() {
		BeforeEach(func() {
			pluginReturn = `{
					"properties":{
						"garden.network.container-ip":"10.255.10.10"
					},
					"search_domains": ["potato", "tomato"]
			  }`
		})

		It("sets the nameserver entries in the container's /etc/resolv.conf to the values supplied by the network plugin", func() {
			Expect(getSearchDomains(container)).To(Equal([]string{"potato", "tomato"}))
		})
	})
})

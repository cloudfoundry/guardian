package gqt_test

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Surviving Restarts", func() {
	Context("when a container is created and then garden is restarted", func() {
		var (
			client              *runner.RunningGarden
			container           garden.Container
			containerBridgeName string
			netOutRules         []garden.NetOutRule
			hostNetInPort       uint32
			interfacePrefix     string
			propertiesDir       string
			containerSpec       garden.ContainerSpec
			restartConfig       runner.GdnRunnerConfig
			gracefulShutdown    bool
			processID           string
			updateContainerFunc func(c garden.Container) error
		)

		BeforeEach(func() {
			propertiesDir = tempDir("", "props")
			config.PropertiesPath = path.Join(propertiesDir, "props.json")
			restartConfig = config

			containerSpec = garden.ContainerSpec{
				Network: fmt.Sprintf("177.100.10.%d/30", 26+GinkgoParallelProcess()*4),
			}

			netOutRules = []garden.NetOutRule{
				garden.NetOutRule{
					Networks: []garden.IPRange{
						garden.IPRangeFromIP(net.ParseIP("8.8.8.8")),
					},
				},
			}

			gracefulShutdown = true
			processID = ""
			updateContainerFunc = nil
		})

		JustBeforeEach(func() {
			client = runner.Start(config)
			var err error
			container, err = client.Create(containerSpec)
			Expect(err).NotTo(HaveOccurred())

			if config.NetworkPluginBin == "" {
				// only makes sense when using the kawasaki networker
				info, err := container.Info()
				Expect(err).NotTo(HaveOccurred())
				interfacePrefix = info.Properties["kawasaki.iptable-prefix"]

				containerBridgeName, err = container.Property("kawasaki.bridge-interface")
				Expect(err).NotTo(HaveOccurred())

				// Sanity check for "destroys the remaining containers' bridges"
				session := gexecStart(exec.Command("ip", "addr", "show", containerBridgeName))
				Expect(session.Wait("10s")).To(gexec.Exit(0))
				Expect(session.Out).To(gbytes.Say("inet %s", info.HostIP))

				hostNetInPort, _, err = container.NetIn(hostNetInPort, 8080)
				Expect(err).NotTo(HaveOccurred())

				Expect(container.BulkNetOut(netOutRules)).To(Succeed())
			}

			if updateContainerFunc != nil {
				Expect(updateContainerFunc(container)).To(Succeed())
			}

			if gracefulShutdown {
				Expect(client.Stop()).To(Succeed())
			} else {
				Expect(client.Kill()).To(MatchError("exit status 137"))
			}

			client = runner.Start(restartConfig)
		})

		AfterEach(func() {
			Expect(os.RemoveAll(propertiesDir)).To(Succeed())
			Expect(client.DestroyAndStop()).To(Succeed())
		})

		Context("when the destroy-containers-on-startup flag is passed", func() {
			JustBeforeEach(func() {
				Eventually(client, time.Second*10).Should(gbytes.Say("guardian.start.clean-up-container.cleaned-up"))
			})

			It("destroys the remaining containers in the depotDir", func() {
				Expect(os.ReadDir(client.DepotDir)).To(BeEmpty())
			})

			It("destroys the remaining containers' iptables", func() {
				out, err := runIPTables("-S", "-t", "filter")
				Expect(err).NotTo(HaveOccurred())
				Expect(string(out)).NotTo(MatchRegexp(fmt.Sprintf("%sinstance.*", interfacePrefix)))
			})

			It("destroys the remaining containers' bridges", func() {
				session := gexecStart(exec.Command("ip", "link", "show", containerBridgeName))
				Expect(session.Wait("10s")).NotTo(gexec.Exit(0))
				Expect(session.Err).To(gbytes.Say("Device \"%s\" does not exist.\n", containerBridgeName))
			})

			It("kills the container processes", func() {
				check := func() string {
					session := gexecStart(exec.Command("sh", "-c", fmt.Sprintf("ps -elf | grep 'while true; do echo %s' | grep -v grep | wc -l", container.Handle())))
					Expect(session.Wait()).To(gexec.Exit(0))
					return string(session.Out.Contents())
				}

				Eventually(check, time.Second*2, time.Millisecond*200).Should(Equal("0\n"), "expected user process to be killed")
				Consistently(check, time.Second*2, time.Millisecond*200).Should(Equal("0\n"), "expected user process to stay dead")
			})

			Context("when running a pea", func() {
				BeforeEach(func() {
					id, err := uuid.NewV4()
					Expect(err).NotTo(HaveOccurred())
					processID = fmt.Sprintf("unique-potato-%s-%d", id, GinkgoParallelProcess())
				})

				Context("with runc", func() {
					BeforeEach(func() {
						skipIfContainerdForProcesses("not relevant to containerd peas")
					})

					It("destroys the pea container", func() {
						Eventually(filepath.Join(getRuncRoot(), processID), "10s").ShouldNot(BeADirectory())
					})
				})

				Context("with containerd", func() {
					BeforeEach(func() {
						skipIfRunDmcForProcesses("not relevant to runc peas")
					})

					It("destroys the peacontainer", func() {
						ctrOutput := func() string {
							return listContainers("ctr", config.ContainerdSocket)
						}
						Eventually(ctrOutput, "20s").ShouldNot(ContainSubstring(processID))
					})
				})
			})

			Context("when the garden server does not shut down gracefully", func() {
				BeforeEach(func() {
					gracefulShutdown = false
				})

				It("destroys orphaned containers' iptables filter rules", func() {
					out, err := runIPTables("-S", "-t", "filter")
					Expect(err).NotTo(HaveOccurred())
					Expect(string(out)).NotTo(MatchRegexp(fmt.Sprintf("%sinstance.*", interfacePrefix)))
				})

				It("destroys orphaned containers' iptables nat rules", func() {
					out, err := runIPTables("-S", "-t", "nat")
					Expect(err).NotTo(HaveOccurred())
					Expect(string(out)).NotTo(MatchRegexp(fmt.Sprintf("%sinstance.*", interfacePrefix)))
				})
			})

			Context("when a container is created after restart", func() {
				It("can be created with the same network reservation", func() {
					_, err := client.Create(containerSpec)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when a container has ICMP NetOut rules applied", func() {
				BeforeEach(func() {
					netOutRules = append(netOutRules,
						garden.NetOutRule{
							Protocol: garden.ProtocolICMP,
							ICMPs: &garden.ICMPControl{
								Type: garden.ICMPType(255),
								Code: garden.ICMPControlCode(uint8(255)),
							},
						})
				})

				It("starts up successfully", func() {
					Expect(client.Ping()).To(Succeed())
				})
			})

			Context("when there is a pea directory without a pid file", func() {
				BeforeEach(func() {
					processDir := filepath.Join(config.DepotDir, "container-handle", "processes", "1234")
					Expect(os.MkdirAll(processDir, os.ModePerm)).To(Succeed())
					Expect(os.WriteFile(filepath.Join(processDir, "config.json"), []byte{}, os.ModePerm)).To(Succeed())
				})

				It("starts up successfully", func() {
					Expect(client.Ping()).To(Succeed())
				})
			})

			Context("when the container is missing the container-type label", func() {
				BeforeEach(func() {
					skipIfNotContainerd()
					updateContainerFunc = func(c garden.Container) error {
						removeContainerLabel(config.ContainerdSocket, c.Handle(), "container-type")
						return nil
					}
				})

				It("deletes the container", func() {
					Expect(listContainers("ctr", config.ContainerdSocket)).NotTo(ContainSubstring(container.Handle()))
				})
			})

		})

		Context("when the destroy-containers-on-startup is true and the networker is temporarily down", func() {
			var failFile *os.File

			BeforeEach(func() {
				config.NetworkPluginBin = binaries.NetworkPlugin

				restartConfig.NetworkPluginBin = binaries.NetworkPlugin
				failFile = tempFile("", "fail")
				updateContainerFunc = func(_ garden.Container) error {
					restartConfig.NetworkPluginExtraArgs = []string{"--fail-once-if-exists", failFile.Name()}
					return nil
				}
			})

			AfterEach(func() {
				os.Remove(failFile.Name())
			})

			It("eventually starts successfully after an initial failure", func() {
				Eventually(client).Should(gbytes.Say("external-networker-result.*exit status 1"))
				Eventually(client).Should(gbytes.Say("guardian.start.clean-up-container.cleaned-up"))
			})
		})

	})
})

func gexecStart(cmd *exec.Cmd) *gexec.Session {
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	return session
}

func removeContainerLabel(socket, handle, label string) {
	runCtr("ctr", socket, []string{"containers", "label", handle, fmt.Sprintf("%s=\"\"", label)})
}

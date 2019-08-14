package gqt_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/cgrouper"
	"code.cloudfoundry.org/guardian/gqt/runner"
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
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
			existingProc        garden.Process
			containerSpec       garden.ContainerSpec
			restartConfig       runner.GdnRunnerConfig
			gracefulShutdown    bool
			processImage        garden.ImageRef
			processID           string
		)

		BeforeEach(func() {
			propertiesDir = tempDir("", "props")
			config.PropertiesPath = path.Join(propertiesDir, "props.json")
			restartConfig = config

			containerSpec = garden.ContainerSpec{
				Network: fmt.Sprintf("177.100.10.%d/30", 26+GinkgoParallelNode()*4),
			}

			netOutRules = []garden.NetOutRule{
				garden.NetOutRule{
					Networks: []garden.IPRange{
						garden.IPRangeFromIP(net.ParseIP("8.8.8.8")),
					},
				},
			}

			gracefulShutdown = true
			processImage = garden.ImageRef{}
			processID = ""
		})

		JustBeforeEach(func() {
			client = runner.Start(config)
			var err error
			container, err = client.Create(containerSpec)
			Expect(err).NotTo(HaveOccurred())

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

			out := gbytes.NewBuffer()
			existingProc, err = container.Run(
				garden.ProcessSpec{
					ID:    processID,
					Path:  "/bin/sh",
					Args:  []string{"-c", fmt.Sprintf("while true; do echo %s; sleep 1; done;", container.Handle())},
					Image: processImage,
				},
				garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, out),
					Stderr: io.MultiWriter(GinkgoWriter, out),
				})
			Expect(err).NotTo(HaveOccurred())

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
			BeforeEach(func() {
				restartConfig.DestroyContainersOnStartup = boolptr(true)
			})

			JustBeforeEach(func() {
				Eventually(client, time.Second*10).Should(gbytes.Say("guardian.start.clean-up-container.cleaned-up"))
			})

			It("destroys the remaining containers in the depotDir", func() {
				Expect(ioutil.ReadDir(client.DepotDir)).To(BeEmpty())
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
					processImage = garden.ImageRef{URI: defaultTestRootFS}
					id, err := uuid.NewV4()
					Expect(err).NotTo(HaveOccurred())
					processID = fmt.Sprintf("unique-potato-%s-%d", id, GinkgoParallelNode())
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
					Expect(ioutil.WriteFile(filepath.Join(processDir, "config.json"), []byte{}, os.ModePerm)).To(Succeed())
				})

				It("starts up successfully", func() {
					Expect(client.Ping()).To(Succeed())
				})
			})
		})

		Context("when the destroy-containers-on-startup flag is not passed", func() {
			Describe("on the pre-existing VM", func() {
				It("does not destroy the depot", func() {
					Expect(filepath.Join(client.DepotDir, container.Handle())).To(BeADirectory())
				})

				It("can still run processes", func() {
					out := gbytes.NewBuffer()
					proc, err := container.Run(
						garden.ProcessSpec{
							Path: "/bin/sh",
							Args: []string{"-c", "echo hello; exit 12"},
						},
						garden.ProcessIO{
							Stdout: io.MultiWriter(GinkgoWriter, out),
							Stderr: io.MultiWriter(GinkgoWriter, out),
						})
					Expect(err).NotTo(HaveOccurred())
					exitCode, err := proc.Wait()
					Expect(err).NotTo(HaveOccurred())

					Expect(exitCode).To(Equal(12))
					Expect(out).To(gbytes.Say("hello"))
				})

				itAllowsTheProcessToContinueRunning := func() {
					It("allows the container process to continue running", func() {
						Consistently(func() string {
							out, err := exec.Command("sh", "-c", fmt.Sprintf("ps -elf | grep 'while true; do echo %s' | grep -v grep | wc -l", container.Handle())).CombinedOutput()
							Expect(err).NotTo(HaveOccurred())
							return string(out)
						}, time.Second*2, time.Millisecond*200).Should(Equal("1\n"), "expected user process to stay alive")
					})
				}
				itAllowsTheProcessToContinueRunning()

				Context("when running a pea", func() {
					BeforeEach(func() {
						processImage = garden.ImageRef{URI: defaultTestRootFS}
					})

					itAllowsTheProcessToContinueRunning()
				})

				It("can reattach to processes that are still running", func() {
					skipIfContainerdForProcesses("Attach with IO is not implemented for pure containerd")
					out := gbytes.NewBuffer()
					process, err := container.Attach(existingProc.ID(), garden.ProcessIO{
						Stdout: io.MultiWriter(GinkgoWriter, out),
						Stderr: io.MultiWriter(GinkgoWriter, out),
					})
					Expect(err).NotTo(HaveOccurred())
					psOutput, err := exec.Command("sh", "-c", "ps -elf | grep /bin/sh").CombinedOutput()
					Expect(err).NotTo(HaveOccurred())
					Eventually(out).Should(
						gbytes.Say(container.Handle()),
						fmt.Sprintf("did not see container handle after 5 seconds.\n/bin/sh processes: %s", string(psOutput)),
					)

					Expect(process.Signal(garden.SignalKill)).To(Succeed())
					_, err = process.Wait()
					Expect(err).NotTo(HaveOccurred())
				})

				It("can still destroy the container", func() {
					Expect(client.Destroy(container.Handle())).To(Succeed())
				})

				It("can still be able to access the internet", func() {
					Expect(checkConnectionWithRetries(container, "8.8.8.8", 53, DEFAULT_RETRIES)).To(Succeed())
				})

				It("can still be accessible from the outside", func() {
					Expect(listenInContainer(container, 8080)).To(Succeed())

					info, err := container.Info()
					Expect(err).NotTo(HaveOccurred())
					externalIP := info.ExternalIP

					// retry because listener process inside other container
					// may not start immediately
					Eventually(func() int {
						session := sendRequest(externalIP, hostNetInPort)
						return session.Wait().ExitCode()
					}).Should(Equal(0))
				})

				It("allows both OCI default and garden specific devices", func() {
					cgroupPath, err := cgrouper.GetCGroupPath(client.CgroupsRootPath(), "devices", config.Tag, containerSpec.Privileged)
					Expect(err).NotTo(HaveOccurred())

					content := readFileString(filepath.Join(cgroupPath, "devices.list"))
					expectedAllowedDevices := []string{
						"c 1:3 rwm",
						"c 5:0 rwm",
						"c 1:8 rwm",
						"c 1:9 rwm",
						"c 1:5 rwm",
						"c 1:7 rwm",
						"c 10:229 rwm",
						"c *:* m",
						"b *:* m",
						"c 5:1 rwm",
						"c 136:* rwm",
						"c 5:2 rwm",
						"c 10:200 rwm",
					}
					contentLines := strings.Split(strings.TrimSpace(content), "\n")
					Expect(contentLines).To(HaveLen(len(expectedAllowedDevices)))
					Expect(contentLines).To(ConsistOf(expectedAllowedDevices))
				})

				Context("when the server denies all the networks", func() {
					BeforeEach(func() {
						config.DenyNetworks = []string{"0.0.0.0/0"}
						restartConfig.DenyNetworks = []string{"0.0.0.0/0"}
					})

					It("still can't access disallowed IPs", func() {
						Expect(checkConnection(container, "8.8.4.4", 53)).NotTo(Succeed())
					})

					It("can still be able to access the allowed IPs", func() {
						Expect(checkConnectionWithRetries(container, "8.8.8.8", 53, DEFAULT_RETRIES)).To(Succeed())
					})
				})

				Context("when the server is restarted without deny networks applied", func() {
					BeforeEach(func() {
						config.DenyNetworks = []string{"0.0.0.0/0"}
						restartConfig.DenyNetworks = []string{}
					})

					It("is able to access the internet", func() {
						Expect(checkConnectionWithRetries(container, "8.8.8.8", 53, DEFAULT_RETRIES)).To(Succeed())
						Expect(checkConnectionWithRetries(container, "8.8.4.4", 53, DEFAULT_RETRIES)).To(Succeed())
					})
				})
			})

			Context("when creating a container after restart", func() {
				It("should not allocate ports used before restart", func() {
					secondContainer, err := client.Create(garden.ContainerSpec{})
					Expect(err).NotTo(HaveOccurred())
					secondContainerHostPort, _, err := secondContainer.NetIn(0, 8080)
					Expect(err).NotTo(HaveOccurred())
					Expect(hostNetInPort).NotTo(Equal(secondContainerHostPort))
				})

				Context("with a subnet used before restart", func() {
					It("will not allocate an IP", func() {
						_, err := client.Create(containerSpec)
						Expect(err).To(MatchError("the requested IP is already allocated"))
					})
				})

				Context("with an IP used before restart", func() {
					BeforeEach(func() {
						containerSpec = garden.ContainerSpec{
							// Specifying a CIDR of < 30 will make garden give us exactly 177.100.10.5
							Network: "177.100.10.5/29",
						}
					})

					It("should not allocate the IP", func() {
						_, err := client.Create(containerSpec)
						Expect(err).To(MatchError("the requested IP is already allocated"))
					})
				})

				Context("with no network specified", func() {
					BeforeEach(func() {
						containerSpec = garden.ContainerSpec{}
					})

					It("successfully creates another container with no network specified", func() {
						_, err := client.Create(containerSpec)
						Expect(err).NotTo(HaveOccurred())
					})
				})
			})
		})
	})
})

func gexecStart(cmd *exec.Cmd) *gexec.Session {
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	return session
}

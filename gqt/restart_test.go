package gqt_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
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
		)

		BeforeEach(func() {
			var err error
			propertiesDir, err = ioutil.TempDir("", "props")
			Expect(err).NotTo(HaveOccurred())
			config.PropertiesPath = path.Join(propertiesDir, "props.json")

			containerSpec = garden.ContainerSpec{
				Network: "177.100.10.30/30",
			}

			netOutRules = []garden.NetOutRule{
				garden.NetOutRule{
					Networks: []garden.IPRange{
						garden.IPRangeFromIP(net.ParseIP("8.8.8.8")),
					},
				},
			}

			restartConfig = defaultConfig()
			gracefulShutdown = true
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
			containerBridgeIPs, err := exec.Command("ip", "addr", "show", containerBridgeName).CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			Expect(string(containerBridgeIPs)).To(ContainSubstring("inet %s", info.HostIP))

			hostNetInPort, _, err = container.NetIn(hostNetInPort, 8080)
			Expect(err).NotTo(HaveOccurred())

			Expect(container.BulkNetOut(netOutRules)).To(Succeed())

			out := gbytes.NewBuffer()
			existingProc, err = container.Run(
				garden.ProcessSpec{
					Path: "/bin/sh",
					Args: []string{"-c", "while true; do echo hello; sleep 1; done;"},
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

			if reflect.DeepEqual(restartConfig, defaultConfig()) {
				restartConfig = config
			}
			client = runner.Start(restartConfig)
		})

		AfterEach(func() {
			Expect(os.RemoveAll(propertiesDir)).To(Succeed())
			Expect(client.DestroyAndStop()).To(Succeed())
		})

		Context("when the destroy-containers-on-startup flag is passed", func() {
			BeforeEach(func() {
				config.DestroyContainersOnStartup = boolptr(true)
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
				var stderr bytes.Buffer
				cmd := exec.Command("ip", "link", "show", containerBridgeName)
				cmd.Stdout = GinkgoWriter
				cmd.Stderr = io.MultiWriter(&stderr, GinkgoWriter)
				Expect(cmd.Run()).NotTo(Succeed())
				Expect(stderr.String()).To(Equal(fmt.Sprintf("Device \"%s\" does not exist.\n", containerBridgeName)))
			})

			It("kills the container processes", func() {
				check := func() string {
					out, err := exec.Command("sh", "-c", "ps -elf | grep 'while true; do echo' | grep -v grep | wc -l").CombinedOutput()
					Expect(err).NotTo(HaveOccurred())
					return string(out)
				}

				Eventually(check, time.Second*2, time.Millisecond*200).Should(Equal("0\n"), "expected user process to stay alive")
				Consistently(check, time.Second*2, time.Millisecond*200).Should(Equal("0\n"), "expected user process to stay alive")
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

				It("allows the container process to continue running", func() {
					Consistently(func() string {
						out, err := exec.Command("sh", "-c", "ps -elf | grep 'while true; do echo' | grep -v grep | wc -l").CombinedOutput()
						Expect(err).NotTo(HaveOccurred())
						return string(out)
					}, time.Second*2, time.Millisecond*200).Should(Equal("1\n"), "expected user process to stay alive")
				})

				It("can reattach to processes that are still running", func() {
					out := gbytes.NewBuffer()
					procId := existingProc.ID()
					process, err := container.Attach(procId, garden.ProcessIO{
						Stdout: io.MultiWriter(GinkgoWriter, out),
						Stderr: io.MultiWriter(GinkgoWriter, out),
					})
					Expect(err).NotTo(HaveOccurred())
					psOutput, err := exec.Command("sh", "-c", "ps -elf | grep /bin/sh").CombinedOutput()
					Expect(err).NotTo(HaveOccurred())
					Eventually(out).Should(
						gbytes.Say("hello"),
						fmt.Sprintf("did not see 'hello' after 5 seconds.\n/bin/sh processes: %s", string(psOutput)),
					)

					Expect(process.Signal(garden.SignalKill)).To(Succeed())
					_, err = process.Wait()
					Expect(err).NotTo(HaveOccurred())
				})

				It("can still destroy the container", func() {
					Expect(client.Destroy(container.Handle())).To(Succeed())
				})

				It("can still be able to access the internet", func() {
					Expect(checkConnection(container, "8.8.8.8", 53)).To(Succeed())
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

				Context("when the server denies all the networks", func() {
					BeforeEach(func() {
						config.DenyNetworks = []string{"0.0.0.0/0"}
					})

					It("still can't access disallowed IPs", func() {
						Expect(checkConnection(container, "8.8.4.4", 53)).NotTo(Succeed())
					})

					It("can still be able to access the allowed IPs", func() {
						Expect(checkConnection(container, "8.8.8.8", 53)).To(Succeed())
					})
				})

				Context("when the server is restarted without deny networks applied", func() {
					BeforeEach(func() {
						restartConfig = config
						config.DenyNetworks = []string{"0.0.0.0/0"}
					})

					It("is able to access the internet", func() {
						Expect(checkConnection(container, "8.8.8.8", 53)).To(Succeed())
						Expect(checkConnection(container, "8.8.4.4", 53)).To(Succeed())
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

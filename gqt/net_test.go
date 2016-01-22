package gqt_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Net", func() {
	var (
		client    *runner.RunningGarden
		container garden.Container

		containerNetwork string
		args             []string

		exampleDotCom net.IP
	)

	BeforeEach(func() {
		args = []string{}
		containerNetwork = fmt.Sprintf("192.168.%d.0/24", 12+GinkgoParallelNode())

		ips, err := net.LookupIP("www.example.com")
		Expect(err).ToNot(HaveOccurred())

		exampleDotCom = ips[0]
	})

	JustBeforeEach(func() {
		var err error

		client = startGarden(args...)

		container, err = client.Create(garden.ContainerSpec{
			Network: containerNetwork,
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	Context("when a network plugin path is provided at startup", func() {
		var (
			tmpFile string
		)

		BeforeEach(func() {
			binPath, err := gexec.Build("github.com/cloudfoundry-incubator/guardian/gqt/cmd/networkplugin")
			Expect(err).NotTo(HaveOccurred())

			tmpDir, err := ioutil.TempDir("", "netplugtest")
			Expect(err).NotTo(HaveOccurred())

			tmpFile = path.Join(tmpDir, "iwasrun.log")

			args = []string{
				"--networkPlugin", binPath,
				"--networkPluginExtraArgs", tmpFile,
			}
		})

		It("executes the network plugin during container creation", func() {
			Expect(tmpFile).To(BeAnExistingFile())
			Expect(ioutil.ReadFile(tmpFile)).To(
				ContainSubstring(
					fmt.Sprintf("up,--handle,%s,--network,%s", container.Handle(), containerNetwork),
				),
			)
		})
	})

	Context("when the native (kawasaki) networker is used", func() {
		It("should include logs from the kawasaki network hook in the main logging output", func() {
			Expect(filepath.Join(client.DepotDir, container.Handle(), "network.log")).To(BeAnExistingFile())
			log, err := ioutil.ReadFile(filepath.Join(client.DepotDir, container.Handle(), "network.log"))
			Expect(err).NotTo(HaveOccurred())
			Expect(gbytes.BufferWithBytes(log)).To(gbytes.Say("kawasaki.hook.start"))
		})

		It("should have a loopback interface", func() {
			buffer := gbytes.NewBuffer()
			proc, err := container.Run(
				garden.ProcessSpec{
					Path: "ifconfig",
					User: "root",
				}, garden.ProcessIO{Stdout: io.MultiWriter(GinkgoWriter, buffer), Stderr: GinkgoWriter},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(proc.Wait()).To(Equal(0))

			Expect(buffer).To(gbytes.Say("lo"))
		})

		It("should have a (dynamically assigned) IP address", func() {
			buffer := gbytes.NewBuffer()
			proc, err := container.Run(
				garden.ProcessSpec{
					Path: "ifconfig",
					User: "root",
				}, garden.ProcessIO{Stdout: io.MultiWriter(GinkgoWriter, buffer), Stderr: io.MultiWriter(GinkgoWriter, buffer)},
			)
			Expect(err).NotTo(HaveOccurred())

			exitCode, err := proc.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(0))

			Expect(buffer).To(gbytes.Say(ipAddress(containerNetwork, 2)))
		})

		It("should be pingable", func() {
			out, err := exec.Command("/bin/ping", "-c 2", ipAddress(containerNetwork, 2)).Output()
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring(" 0% packet loss"))
		})

		Context("a second container", func() {
			var originContainer garden.Container

			JustBeforeEach(func() {
				var err error
				originContainer = container
				container, err = client.Create(garden.ContainerSpec{
					Network: containerNetwork,
				})

				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				Expect(client.Destroy(originContainer.Handle())).To(Succeed())
			})

			It("should have the next IP address", func() {
				buffer := gbytes.NewBuffer()
				proc, err := container.Run(
					garden.ProcessSpec{
						Path: "ifconfig",
						User: "root",
					}, garden.ProcessIO{Stdout: buffer},
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(proc.Wait()).To(Equal(0))

				Expect(buffer).To(gbytes.Say(ipAddress(containerNetwork, 3)))
			})

			It("should be pingable", func() {
				out, err := exec.Command("/bin/ping", "-c 2", ipAddress(containerNetwork, 2)).Output()
				Expect(out).To(ContainSubstring(" 0% packet loss"))
				Expect(err).ToNot(HaveOccurred())

				out, err = exec.Command("/bin/ping", "-c 2", ipAddress(containerNetwork, 3)).Output()
				Expect(out).To(ContainSubstring(" 0% packet loss"))
				Expect(err).ToNot(HaveOccurred())
			})

			It("should access internet", func() {
				Expect(checkConnection(container, exampleDotCom.String(), 80)).To(Succeed())
			})
		})

		Context("when default network pool is changed", func() {
			var (
				otherContainer   garden.Container
				otherContainerIP string
			)

			BeforeEach(func() {
				args = []string{"-networkPool", "10.254.0.0/29"}
				containerNetwork = ""
			})

			JustBeforeEach(func() {
				var err error
				otherContainer, err = client.Create(garden.ContainerSpec{})
				Expect(err).ToNot(HaveOccurred())

				otherContainerIP = containerIP(otherContainer)

				Expect(client.Destroy(otherContainer.Handle())).To(Succeed())

				otherContainer, err = client.Create(garden.ContainerSpec{})
				Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				Expect(client.Destroy(otherContainer.Handle())).To(Succeed())
			})

			It("reuses IP addresses", func() {
				newIpAddress := containerIP(otherContainer)
				Expect(newIpAddress).To(Equal(otherContainerIP))
			})

			It("is accessible from the outside", func() {
				hostPort, containerPort, err := otherContainer.NetIn(0, 4321)
				Expect(err).ToNot(HaveOccurred())

				Expect(listenInContainer(otherContainer, containerPort)).To(Succeed())

				externalIP := externalIP(otherContainer)
				stdout := sendRequest(externalIP, hostPort)
				Expect(stdout).To(gbytes.Say(fmt.Sprintf("%d", containerPort)))
			})
		})

		Describe("--denyNetworks flag", func() {
			BeforeEach(func() {
				args = append(args, "--denyNetworks", "8.8.8.0/24")
			})

			It("should deny outbound traffic to IPs in the range", func() {
				Expect(checkConnection(container, "8.8.8.8", 53)).To(MatchError("Request failed. Process exited with code 1"))
			})

			It("should allow outbound traffic to IPs outside of the range", func() {
				Expect(checkConnection(container, "8.8.4.4", 53)).To(Succeed())
			})

			Context("when multiple denyNetworks are defined", func() {
				BeforeEach(func() {
					args = append(args, "--denyNetworks", "8.8.8.0/24,8.8.4.0/24")
				})

				It("should deny IPs in either range", func() {
					Expect(checkConnection(container, "8.8.8.8", 53)).To(MatchError("Request failed. Process exited with code 1"))
					Expect(checkConnection(container, "8.8.4.4", 53)).To(MatchError("Request failed. Process exited with code 1"))
				})
			})
		})

		Describe("NetIn", func() {
			It("maps the provided host port to the container port", func() {
				const (
					hostPort      uint32 = 9888
					containerPort uint32 = 9080
				)

				actualHostPort, actualContainerPort, err := container.NetIn(hostPort, containerPort)
				Expect(err).ToNot(HaveOccurred())

				Expect(actualHostPort).To(Equal(hostPort))
				Expect(actualContainerPort).To(Equal(containerPort))
				Expect(listenInContainer(container, containerPort)).To(Succeed())

				externalIP := externalIP(container)
				stdout := sendRequest(externalIP, hostPort)
				Expect(stdout).To(gbytes.Say(fmt.Sprintf("%d", containerPort)))
			})

			It("maps the provided host port to the container port", func() {
				actualHostPort, actualContainerPort, err := container.NetIn(0, 0)
				Expect(err).ToNot(HaveOccurred())

				Expect(actualHostPort).NotTo(Equal(0))
				Expect(actualContainerPort).NotTo(Equal(0))
				Expect(listenInContainer(container, actualContainerPort)).To(Succeed())

				externalIP := externalIP(container)
				stdout := sendRequest(externalIP, actualHostPort)
				Expect(stdout).To(gbytes.Say(fmt.Sprintf("%d", actualContainerPort)))
			})
		})

		Describe("NetOut", func() {
			BeforeEach(func() {
				args = append(args, "--denyNetworks", "0.0.0.0/0")
			})

			It("should access internet", func() {
				Expect(checkConnection(container, "8.8.8.8", 53)).To(MatchError("Request failed. Process exited with code 1"))

				Expect(container.NetOut(garden.NetOutRule{
					Protocol: garden.ProtocolTCP,
					Networks: []garden.IPRange{garden.IPRangeFromIP(net.ParseIP("8.8.8.8"))},
					Ports:    []garden.PortRange{garden.PortRangeFromPort(53)},
				})).To(Succeed())

				Expect(checkConnection(container, "8.8.8.8", 53)).To(Succeed())
			})

			Context("external addresses", func() {
				var (
					ByAllowingTCP, ByRejectingTCP func()
				)

				BeforeEach(func() {
					ByAllowingTCP = func() {
						By("allowing outbound tcp traffic", func() {
							Expect(checkConnection(container, exampleDotCom.String(), 80)).To(Succeed())
						})
					}

					ByRejectingTCP = func() {
						By("rejecting outbound tcp traffic", func() {
							Expect(checkConnection(container, exampleDotCom.String(), 80)).NotTo(Succeed())
						})
					}
				})

				Context("when the target address is inside DENY_NETWORKS", func() {
					//The target address is the ip addr of www.example.com in these tests
					BeforeEach(func() {
						args = append(args, "--denyNetworks", "0.0.0.0/0")
						containerNetwork = fmt.Sprintf("10.1%d.0.0/24", GinkgoParallelNode())
					})

					It("disallows TCP connections", func() {
						ByRejectingTCP()
					})

					Context("when a rule that allows all traffic to the target is added", func() {
						JustBeforeEach(func() {
							err := container.NetOut(garden.NetOutRule{
								Networks: []garden.IPRange{
									garden.IPRangeFromIP(exampleDotCom),
								},
							})
							Expect(err).ToNot(HaveOccurred())
						})

						It("allows TCP traffic to the target", func() {
							ByAllowingTCP()
						})
					})
				})

				Context("when the target address is not in DENY_NETWORKS", func() {
					BeforeEach(func() {
						args = append(args, "--denyNetworks", "4.4.4.4/30")
						containerNetwork = fmt.Sprintf("10.1%d.0.0/24", GinkgoParallelNode())
					})

					It("allows connections", func() {
						ByAllowingTCP()
					})
				})
			})
		})
	})
})

func externalIP(container garden.Container) string {
	properties, err := container.Properties()
	Expect(err).NotTo(HaveOccurred())
	return properties["kawasaki.external-ip"]
}

func containerIP(container garden.Container) string {
	properties, err := container.Properties()
	Expect(err).NotTo(HaveOccurred())
	return properties["kawasaki.container-ip"]
}

func checkConnection(container garden.Container, ip string, port int) error {
	process, err := container.Run(garden.ProcessSpec{
		User: "alice",
		Path: "sh",
		Args: []string{"-c", fmt.Sprintf("echo hello | nc -w1 %s %d", ip, port)},
	}, garden.ProcessIO{Stdout: GinkgoWriter, Stderr: GinkgoWriter})
	if err != nil {
		return err
	}

	exitCode, err := process.Wait()
	if err != nil {
		return err
	}

	if exitCode == 0 {
		return nil
	} else {
		return fmt.Errorf("Request failed. Process exited with code %d", exitCode)
	}
}

func ipAddress(subnet string, index int) string {
	ip := strings.Split(subnet, "/")[0]
	pattern := regexp.MustCompile(".[0-9]+$")
	ip = pattern.ReplaceAllString(ip, fmt.Sprintf(".%d", index))
	return ip
}

func listenInContainer(container garden.Container, containerPort uint32) error {
	_, err := container.Run(garden.ProcessSpec{
		User: "alice",
		Path: "sh",
		Args: []string{"-c", fmt.Sprintf("echo %d | nc -l -p %d", containerPort, containerPort)},
	}, garden.ProcessIO{
		Stdout: GinkgoWriter,
		Stderr: GinkgoWriter,
	})
	Expect(err).ToNot(HaveOccurred())
	time.Sleep(2 * time.Second)

	return err
}

func sendRequest(ip string, port uint32) *gbytes.Buffer {
	stdout := gbytes.NewBuffer()
	cmd := exec.Command("nc", "-w3", ip, fmt.Sprintf("%d", port))
	cmd.Stdout = stdout
	cmd.Stderr = GinkgoWriter

	err := cmd.Run()
	Expect(err).ToNot(HaveOccurred())

	return stdout
}

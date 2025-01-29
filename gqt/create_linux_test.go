package gqt_test

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/cgrouper"
	"code.cloudfoundry.org/guardian/gqt/runner"
	"code.cloudfoundry.org/guardian/guardiancmd"
	gardencgroups "code.cloudfoundry.org/guardian/rundmc/cgroups"
	"code.cloudfoundry.org/guardian/rundmc/sysctl"

	. "code.cloudfoundry.org/guardian/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Creating a Container", func() {
	var (
		client    *runner.RunningGarden
		container garden.Container

		initialSockets int
		initialPipes   int
	)

	JustBeforeEach(func() {
		client = runner.Start(config)
		initialSockets = numOpenSockets(client.Pid)
		initialPipes = numPipes(client.Pid)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	It("has the expected device list allowed", func() {
		if gardencgroups.IsCgroup2UnifiedMode() {
			Skip("Skipping cgroups v1 tests when cgroups v2 is enabled")
		}

		var err error
		container, err = client.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())

		parentPath, err := cgrouper.GetCGroupPath(client.CgroupsRootPath(), "devices", strconv.Itoa(GinkgoParallelProcess()), false, cpuThrottlingEnabled())
		Expect(err).NotTo(HaveOccurred())
		cgroupPath := filepath.Join(parentPath, container.Handle())

		content := readFileString(filepath.Join(cgroupPath, "devices.list"))
		expectedAllowedDevices := []string{
			"c 1:3 rwm",
			"c 5:0 rwm",
			"c 1:8 rwm",
			"c 1:9 rwm",
			"c 1:5 rwm",
			"c 1:7 rwm",
			"c *:* m",
			"b *:* m",
			"c 136:* rwm",
			"c 5:2 rwm",
		}
		contentLines := strings.Split(strings.TrimSpace(content), "\n")
		Expect(contentLines).To(HaveLen(len(expectedAllowedDevices)))
		Expect(contentLines).To(ConsistOf(expectedAllowedDevices))
	})

	Context("when creating fails", func() {
		// cause Create to fail by specifying an invalid network CIDR address
		containerSpec := garden.ContainerSpec{
			Network: "not-a-valid-network",
		}

		It("returns a nice error rather than timing out", func() {
			_, err := client.Create(containerSpec)
			Expect(err).To(MatchError(ContainSubstring("invalid CIDR address")))
		})

		It("cleans up the depot directory", func() {
			_, err := client.Create(containerSpec)
			Expect(err).To(HaveOccurred())

			Expect(os.ReadDir(client.DepotDir)).To(BeEmpty())
		})

		It("cleans up the groot store", func() {
			// pre-warm cache to avoid test pollution
			// i.e. ensure base layers that are never removed are already in the groot store
			_, err := client.Create(containerSpec)
			Expect(err).To(HaveOccurred())

			prev, err := os.ReadDir(filepath.Join(client.TmpDir, "groot_store", "images"))
			Expect(err).NotTo(HaveOccurred())

			_, err = client.Create(containerSpec)
			Expect(err).To(HaveOccurred())

			Eventually(func() int {
				num, err := os.ReadDir(filepath.Join(client.TmpDir, "groot_store", "images"))
				Expect(err).NotTo(HaveOccurred())
				return len(num)
			}).Should(Equal(len(prev)))
		})

		Context("because runc doesn't exist", func() {
			BeforeEach(func() {
				skipIfContainerd()
				config.RuntimePluginBin = "/tmp/does/not/exist"
			})

			It("returns a sensible error", func() {
				_, err := client.Create(garden.ContainerSpec{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no such file or directory"))
			})
		})
	})

	Context("after creating a container without a specified handle", func() {
		var (
			privileged bool

			initProcPid int
		)

		BeforeEach(func() {
			privileged = false
		})

		JustBeforeEach(func() {
			var err error
			container, err = client.Create(garden.ContainerSpec{
				Privileged: privileged,
			})
			Expect(err).NotTo(HaveOccurred())

			initProcPid = initProcessPID(container.Handle())
		})

		It("should create a depot subdirectory based on the container handle", func() {
			skipIfContainerd()
			Expect(container.Handle()).NotTo(BeEmpty())
			Expect(filepath.Join(client.DepotDir, container.Handle())).To(BeADirectory())
			Expect(filepath.Join(client.DepotDir, container.Handle(), "config.json")).To(BeARegularFile())
		})

		It("should lookup the right container", func() {
			lookupContainer, lookupError := client.Lookup(container.Handle())

			Expect(lookupError).NotTo(HaveOccurred())
			Expect(lookupContainer).To(Equal(container))
		})

		It("should not leak pipes", func() {
			process, err := container.Run(garden.ProcessSpec{Path: "echo", Args: []string{"hello"}}, garden.ProcessIO{})
			Expect(err).NotTo(HaveOccurred())

			Expect(process.Wait()).To(Equal(0))

			Expect(client.Destroy(container.Handle())).To(Succeed())
			container = nil // avoid double-destroying

			Eventually(func() int { return numPipes(client.Pid) }).Should(Equal(initialPipes))
		})

		It("should not leak sockets", func() {
			Expect(client.Destroy(container.Handle())).To(Succeed())
			container = nil // avoid double-destroying

			Eventually(func() int { return numOpenSockets(client.Pid) }).Should(Equal(initialSockets))
		})

		It("should avoid leaving zombie processes", func() {
			Expect(client.Destroy(container.Handle())).To(Succeed())
			container = nil // avoid double-destroying

			Eventually(func() *gexec.Session {
				sess, err := gexec.Start(exec.Command("ps"), GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))
				return sess
			}, "10s").ShouldNot(gbytes.Say("defunct")) // this is a pretty broad test since we're looking at all processes, so give it quite a while to see no defuncts
		})

		DescribeTable("placing the container in to all namespaces", func(ns string) {
			hostNSInode, err := os.Readlink(fmt.Sprintf("/proc/1/ns/%s", ns))
			Expect(err).NotTo(HaveOccurred())

			containerNSInode, err := os.Readlink(fmt.Sprintf("/proc/%d/ns/%s", initProcPid, ns))
			Expect(err).NotTo(HaveOccurred())

			Expect(hostNSInode).NotTo(Equal(containerNSInode))
		},
			Entry("should place the container in to the NET namespace", "net"),
			Entry("should place the container in to the IPC namespace", "ipc"),
			Entry("should place the container in to the UTS namespace", "uts"),
			Entry("should place the container in to the PID namespace", "pid"),
			Entry("should place the container in to the MNT namespace", "mnt"),
			Entry("should place the container in to the USER namespace", "user"),
		)

		Context("which is privileged", func() {
			BeforeEach(func() {
				privileged = true
			})

			It("should not place the container in its own user namespace", func() {
				hostNS, err := gexec.Start(exec.Command("ls", "-l", "/proc/1/ns/user"), GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				containerNS, err := gexec.Start(exec.Command("ls", "-l", fmt.Sprintf("/proc/%d/ns/user", initProcPid)), GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(containerNS).Should(gexec.Exit(0))
				Eventually(hostNS).Should(gexec.Exit(0))

				hostFD := strings.Split(string(hostNS.Out.Contents()), ">")[1]
				containerFD := strings.Split(string(containerNS.Out.Contents()), ">")[1]

				Expect(hostFD).To(Equal(containerFD))
			})
		})
	})

	Context("after creating a container with a specified root filesystem", func() {
		var (
			tmpDir     string
			rootFSPath string
		)

		JustBeforeEach(func() {
			var err error

			rootFSPath = createRootfsTar(func(unpackedRootfs string) {
				Expect(os.WriteFile(filepath.Join(unpackedRootfs, "my-file"), []byte("some-content"), 0644)).To(Succeed())
				Expect(os.Mkdir(path.Join(unpackedRootfs, "somedir"), 0777)).To(Succeed())
			})

			container, err = client.Create(garden.ContainerSpec{
				RootFSPath: rootFSPath,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(filepath.Dir(rootFSPath))).To(Succeed())
			Expect(os.RemoveAll(tmpDir)).To(Succeed())
		})

		It("provides the containers with the right rootfs", func() {
			Expect(container).To(HaveFile("/my-file"))

			By("Isolating the filesystem propertly for multiple containers")

			runInContainer(container, "touch", []string{"/somedir/created-file"})
			Expect(container).To(HaveFile("/somedir/created-file"))

			container2, err := client.Create(garden.ContainerSpec{
				RootFSPath: rootFSPath,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(container2).To(HaveFile("/my-file"))
			Expect(container2).NotTo(HaveFile("/somedir/created-file"))
		})
	})

	Context("after creating a container with a specified handle", func() {
		It("should lookup the right container for the handle", func() {
			container, err := client.Create(garden.ContainerSpec{
				Handle: "container-banana",
			})
			Expect(err).NotTo(HaveOccurred())

			lookupContainer, lookupError := client.Lookup("container-banana")
			Expect(lookupError).NotTo(HaveOccurred())
			Expect(lookupContainer).To(Equal(container))
		})

		It("allow the container to be created with the same name after destroying", func() {
			container, err := client.Create(garden.ContainerSpec{
				Handle: "another-banana",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(client.Destroy(container.Handle())).To(Succeed())

			_, err = client.Create(garden.ContainerSpec{
				Handle: "another-banana",
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
	// TODO why duplicate?
	Context("when creating a container fails", func() {
		It("should not leak networking configuration", func() {
			_, err := client.Create(garden.ContainerSpec{
				Network:    fmt.Sprintf("172.250.%d.20/24", GinkgoParallelProcess()),
				RootFSPath: "/banana/does/not/exist",
			})
			Expect(err).To(HaveOccurred())

			session, err := gexec.Start(
				exec.Command("ip", "addr"),
				GinkgoWriter, GinkgoWriter,
			)
			Expect(err).NotTo(HaveOccurred())
			Consistently(session).ShouldNot(gbytes.Say(fmt.Sprintf("172.250.%d.0", GinkgoParallelProcess())))
		})
	})

	Context("when creating a container with NetOut rules", func() {
		var container garden.Container

		JustBeforeEach(func() {
			config.DenyNetworks = []string{"0.0.0.0/0"}

			rules := []garden.NetOutRule{
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

			var err error
			container, err = client.Create(garden.ContainerSpec{
				NetOut: rules,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("provides connectivity to the addresses provided", func() {
			Expect(checkConnectionWithRetries(container, "8.8.8.8", 53, DEFAULT_RETRIES)).To(Succeed())
			Expect(checkConnectionWithRetries(container, "8.8.4.4", 53, DEFAULT_RETRIES)).To(Succeed())
		})
	})

	Context("when creating a container with NetIn rules", func() {
		var container garden.Container

		JustBeforeEach(func() {
			netIn := []garden.NetIn{
				garden.NetIn{HostPort: 9888, ContainerPort: 9080},
			}

			var err error
			container, err = client.Create(garden.ContainerSpec{
				NetIn: netIn,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("maps the provided host port to the container port", func() {
			Expect(listenInContainer(container, 9080)).To(Succeed())

			externalIP := externalIP(container)

			serverMustReply(externalIP, 9888, "9080")
		})
	})

	Context("when creating a container and specifying CPU configuration", func() {
		createContainerWithCpuConfig := func(weight, shares uint64) (garden.Container, error) {
			limits := garden.Limits{
				CPU: garden.CPULimits{
					Weight:        weight,
					LimitInShares: shares,
				},
			}

			container, err := client.Create(garden.ContainerSpec{
				Limits: limits,
			})

			return container, err
		}

		getContainerCPUShares := func(container garden.Container) int {
			cpuSharesFile := "cpu.shares"
			if gardencgroups.IsCgroup2UnifiedMode() {
				cpuSharesFile = "cpu.weight"
			}
			cpuSharesPath := filepath.Join(client.CgroupSubsystemPath("cpu", container.Handle()), cpuSharesFile)
			cpuShares := strings.TrimSpace(readFileString(cpuSharesPath))
			numShares, err := strconv.Atoi(cpuShares)
			Expect(err).NotTo(HaveOccurred())
			return numShares
		}

		Context("cgroups v1", func() {
			BeforeEach(func() {
				if gardencgroups.IsCgroup2UnifiedMode() {
					Skip("Skipping cgroups v1 tests when cgroups v2 is enabled")
				}
			})

			It("can set the cpu weight", func() {
				container, err := createContainerWithCpuConfig(2, 0)
				Expect(err).NotTo(HaveOccurred())

				Expect(getContainerCPUShares(container)).To(Equal(2))
			})

			It("should return an error when the cpu shares is invalid", func() {
				_, err := createContainerWithCpuConfig(1, 0)

				Expect(err.Error()).To(ContainSubstring("minimum allowed cpu-shares is 2"))
			})

			It("should use the default weight value when neither the cpu share or weight are set", func() {
				container, err := createContainerWithCpuConfig(0, 0)
				Expect(err).NotTo(HaveOccurred())
				Expect(getContainerCPUShares(container)).To(Equal(1024))
			})

			Context("when LimitInShares is set", func() {
				It("creates a container with the shares", func() {
					container, err := createContainerWithCpuConfig(0, 123)
					Expect(err).NotTo(HaveOccurred())
					Expect(getContainerCPUShares(container)).To(Equal(123))
				})
			})

			Context("when both Weight and LimitInShares are set", func() {
				It("Weight has precedence", func() {
					container, err := createContainerWithCpuConfig(123, 456)
					Expect(err).NotTo(HaveOccurred())
					Expect(getContainerCPUShares(container)).To(Equal(123))
				})
			})
		})

		Context("cgroups v2", func() {
			BeforeEach(func() {
				if !gardencgroups.IsCgroup2UnifiedMode() {
					Skip("Skipping cgroups v2 tests when cgroups v1 is enabled")
				}
			})

			It("can set the cpu weight", func() {
				container, err := createContainerWithCpuConfig(2, 0)
				Expect(err).NotTo(HaveOccurred())

				Expect(getContainerCPUShares(container)).To(Equal(1))
			})

			It("should return an error when the cpu shares is invalid", func() {
				_, err := createContainerWithCpuConfig(1, 0)

				Expect(err.Error()).To(ContainSubstring("numerical result out of range"))
			})

			It("should use the default weight value when neither the cpu share or weight are set", func() {
				container, err := createContainerWithCpuConfig(0, 0)
				Expect(err).NotTo(HaveOccurred())
				Expect(getContainerCPUShares(container)).To(Equal(100))
			})

			Context("when LimitInShares is set", func() {
				It("creates a container with the shares", func() {
					container, err := createContainerWithCpuConfig(0, 123)
					Expect(err).NotTo(HaveOccurred())
					Expect(getContainerCPUShares(container)).To(Equal(5))
				})
			})

			Context("when both Weight and LimitInShares are set", func() {
				It("Weight has precedence", func() {
					container, err := createContainerWithCpuConfig(123, 456)
					Expect(err).NotTo(HaveOccurred())
					Expect(getContainerCPUShares(container)).To(Equal(5))
				})
			})
		})
	})

	Describe("block IO weight", func() {
		BeforeEach(func() {
			kernelMinVersionChecker := guardiancmd.NewKernelMinVersionChecker(sysctl.New())
			is50, err := kernelMinVersionChecker.CheckVersionIsAtLeast(5, 0, 0)
			Expect(err).NotTo(HaveOccurred())
			if is50 {
				Skip("blkio.weight is removed in kernels >= 5.0")
			}
			config.DefaultBlkioWeight = uint64ptr(400)
		})

		getContainerBlockIOWeight := func(container garden.Container) int {
			blkioWeightPath := filepath.Join(client.CgroupSubsystemPath("blkio", container.Handle()), "blkio.weight")
			blkIOWeight := strings.TrimSpace(readFileString(blkioWeightPath))
			numBlkioWeight, err := strconv.Atoi(blkIOWeight)
			Expect(err).NotTo(HaveOccurred())
			return numBlkioWeight
		}

		It("uses the specified block IO weight", func() {
			container, err := client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())
			Expect(getContainerBlockIOWeight(container)).To(Equal(400))
		})

		Context("when specifying a block IO weight of 0", func() {
			BeforeEach(func() {
				config.DefaultBlkioWeight = uint64ptr(0)
			})

			It("uses the system default value of 500", func() {
				container, err := client.Create(garden.ContainerSpec{})
				Expect(err).NotTo(HaveOccurred())
				Expect(getContainerBlockIOWeight(container)).To(Equal(500))
			})
		})

		Context("when specifying block IO weight outside the range 10 - 1000", func() {
			BeforeEach(func() {
				config.DefaultBlkioWeight = uint64ptr(9)
			})

			It("returns an out of range error", func() {
				_, err := client.Create(garden.ContainerSpec{})
				Expect(err.Error()).To(ContainSubstring("numerical result out of range"))
			})
		})
	})

	Context("when running with an external network plugin", func() {
		var pluginOutput string
		BeforeEach(func() {
			config.NetworkPluginBin = binaries.NetworkPlugin
		})

		Context("when the plugin returns a properties key", func() {
			BeforeEach(func() {
				pluginOutput = `{"properties": {"key":"value", "garden.network.container-ip":"10.10.24.3"}}`
				config.NetworkPluginExtraArgs = append(config.NetworkPluginExtraArgs, "--output", pluginOutput)
			})

			It("does not run kawasaki", func() {
				container, err := client.Create(garden.ContainerSpec{})
				Expect(err).NotTo(HaveOccurred())

				out := gbytes.NewBuffer()
				process, err := container.Run(garden.ProcessSpec{
					Path: "ip",
					Args: []string{
						"-o",
						"link",
						"show",
					},
				}, garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, out),
				})
				Expect(err).NotTo(HaveOccurred())

				exitCode, err := process.Wait()
				Expect(err).NotTo(HaveOccurred())
				Expect(exitCode).To(BeZero())

				// ip link appends a new line on the end so let's trim that first
				contents := strings.TrimRight(string(out.Contents()), "\n")

				// Check that we only have 1 interface, the loopback interface
				Expect(strings.Split(contents, "\n")).To(HaveLen(1))
				Expect(contents).To(ContainSubstring("LOOPBACK"))
			})
		})

		Context("when the external network plugin returns invalid JSON", func() {
			BeforeEach(func() {
				pluginOutput = "invalid-json"
				config.NetworkPluginExtraArgs = append(config.NetworkPluginExtraArgs, "--output", pluginOutput)
			})

			It("returns a useful error message", func() {
				_, err := client.Create(garden.ContainerSpec{})
				Expect(err).To(MatchError(ContainSubstring("unmarshaling result from external networker: invalid character")))
			})
		})
	})

	It("does not make containers available to lookup until creation is completed", func() {
		handle := "handlecake"

		assertionsComplete := make(chan struct{})
		go func(done chan<- struct{}) {
			defer GinkgoRecover()
			defer close(done)

			var lookupContainer garden.Container
			Eventually(func() error {
				var err error
				lookupContainer, err = client.Lookup(handle)
				return err
			}, time.Second*20, time.Millisecond*200).ShouldNot(HaveOccurred())

			// Properties used to be set after containers were available from lookup
			Expect(lookupContainer.Properties()).To(HaveKeyWithValue("somename", "somevalue"))
		}(assertionsComplete)

		_, err := client.Create(garden.ContainerSpec{
			Handle:     handle,
			Properties: garden.Properties{"somename": "somevalue"},
		})
		Expect(err).NotTo(HaveOccurred())

		<-assertionsComplete
	})

	Context("create more containers than the maxkeyring limit", func() {
		BeforeEach(func() {
			Expect(os.WriteFile("/proc/sys/kernel/keys/maxkeys", []byte("1"), 0644)).To(Succeed())
		})

		AfterEach(func() {
			Expect(os.WriteFile("/proc/sys/kernel/keys/maxkeys", []byte("200"), 0644)).To(Succeed())
		})

		It("works", func() {
			c1, err := client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			c2, err := client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			Expect(client.Destroy(c1.Handle())).To(Succeed())
			Expect(client.Destroy(c2.Handle())).To(Succeed())
		})
	})

	Context("when creating more than --max-containers containers", func() {
		BeforeEach(func() {
			config.MaxContainers = uint64ptr(1)
		})

		JustBeforeEach(func() {
			_, err := client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error", func() {
			_, err := client.Create(garden.ContainerSpec{})
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(("max containers reached")))
		})
	})

	Describe("creating privileged containers", func() {
		Context("when --disable-privileged-containers is not specified", func() {
			It("can create privileged containers", func() {
				_, err := client.Create(garden.ContainerSpec{Privileged: true})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when --disable-privileged-containers is set", func() {
			BeforeEach(func() {
				config.DisablePrivilegedContainers = boolptr(true)
			})

			It("cannot create privileged containers, even when gdn runs as root", func() {
				_, err := client.Create(garden.ContainerSpec{Privileged: true})
				Expect(err).To(MatchError("privileged container creation is disabled"))
			})
		})
	})
})

func psauxf() string {
	out, err := exec.Command("ps", "auxf").CombinedOutput()
	Expect(err).NotTo(HaveOccurred())
	return fmt.Sprintf("\nPS OUTPUT:\n%s\n", string(out))
}

func numOpenSockets(pid int) (num int) {
	stdout := runCommand(exec.Command("sh", "-c", fmt.Sprintf("lsof -p %d | grep sock", pid)), psauxf)
	return strings.Count(stdout, "\n")
}

func numPipes(pid int) (num int) {
	stdout := runCommand(exec.Command("sh", "-c", fmt.Sprintf("lsof -p %d | grep pipe", pid)), psauxf)
	return strings.Count(stdout, "\n")
}

package gqt_test

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/containerdrunner"
	"code.cloudfoundry.org/guardian/gqt/runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Destroying a Container", func() {
	var (
		client *runner.RunningGarden
	)

	BeforeEach(func() {
		config.DebugIP = "0.0.0.0"
		config.DebugPort = intptr(8080 + GinkgoParallelNode())
	})

	JustBeforeEach(func() {
		client = runner.Start(config)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	It("should not leak goroutines", func() {
		var numGoRoutines = func() int {
			numGoroutines, err := client.NumGoroutines()
			Expect(err).NotTo(HaveOccurred())
			return numGoroutines
		}

		numGoroutinesBefore := numGoRoutines()

		handle := fmt.Sprintf("goroutine-leak-test-%d", GinkgoParallelNode())
		_, err := client.Create(garden.ContainerSpec{
			Handle: handle,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(client.Destroy(handle)).To(Succeed())

		Eventually(numGoRoutines).Should(BeNumerically("<=", numGoroutinesBefore))
	})

	It("should destroy the container's rootfs", func() {
		container, err := client.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())

		info, err := container.Info()
		Expect(err).NotTo(HaveOccurred())
		containerRootfs := info.ContainerPath

		Expect(client.Destroy(container.Handle())).To(Succeed())

		Expect(containerRootfs).NotTo(BeAnExistingFile())
	})

	It("should destroy the container's depot directory", func() {
		container, err := client.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())

		Expect(client.Destroy(container.Handle())).To(Succeed())

		Expect(filepath.Join(client.DepotDir, container.Handle())).NotTo(BeAnExistingFile())
	})

	It("should kill the container's init process", func() {
		container, err := client.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())

		initProcPid := initProcessPID(container.Handle())

		_, err = container.Run(garden.ProcessSpec{
			Path: "/bin/sh",
			Args: []string{
				"-c", "read x",
			},
		}, ginkgoIO)
		Expect(err).NotTo(HaveOccurred())

		Expect(client.Destroy(container.Handle())).To(Succeed())

		var killExitCode = func() int {
			sess, err := gexec.Start(exec.Command("kill", "-0", fmt.Sprintf("%d", initProcPid)), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			sess.Wait(1 * time.Second)
			return sess.ExitCode()
		}

		Eventually(killExitCode).Should(Equal(1))
	})

	Context("when container destroy is interrupted half way through", func() {
		var originalConfig runner.GdnRunnerConfig

		BeforeEach(func() {
			tmpDir, err := ioutil.TempDir("", "netplugtest")
			Expect(err).NotTo(HaveOccurred())

			argsFile := path.Join(tmpDir, "args.log")
			stdinFile := path.Join(tmpDir, "stdin.log")

			pluginReturn := `{"properties":{
					"garden.network.container-ip":"10.255.10.10",
					"garden.network.host-ip":"255.255.255.255"
				}}`

			config.PropertiesPath = path.Join(tmpDir, "props.json")
			config.NetworkPluginBin = binaries.NetworkPlugin
			// simulate this scenario by starting guardian with a network plugin which
			// kill -9s <guardian pid> on 'down' (i.e. half way through a container delete)
			// then, start the guardian server backup without the plugin, and ensuring that
			// --destroy-containers-on-startup=false
			config.NetworkPluginExtraArgs = []string{argsFile, stdinFile, pluginReturn}
			originalConfig = config
			config.NetworkPluginExtraArgs = append(config.NetworkPluginExtraArgs, "kill-garden-server")
		})

		It("leaves the bundle dir in the depot", func() {
			container, err := client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			Expect(client.Destroy(container.Handle())).NotTo(Succeed())
			Eventually(client).Should(gexec.Exit())
			// This sleep is here because it helps avoid what looks like a race condition in cgroup removal vs
			// writing to the devices.deny file on startup. Without it, we frequently hit a condition where
			// listing directories under the `garden` cgroup returns nothing, but writing to `devices.deny`
			// returns with an EINVAL (indicative of there being cgroup children). Possibly a kernel race?
			time.Sleep(time.Second)

			// start guardian back up with the 'kill -9 <gdn pid> on down' behaviour disabled
			client = runner.Start(originalConfig)

			bundleDir := filepath.Join(client.DepotDir, container.Handle())
			Expect(bundleDir).To(BeADirectory())

			Expect(client.Destroy(container.Handle())).To(Succeed())

			bundleDir = filepath.Join(client.DepotDir, container.Handle())
			Expect(bundleDir).NotTo(BeADirectory())
		})
	})

	Describe("networking resources", func() {
		var (
			container         garden.Container
			networkSpec       string
			contIfaceName     string
			networkBridgeName string
		)

		JustBeforeEach(func() {
			var err error

			networkSpec = fmt.Sprintf("177.100.%d.0/24", GinkgoParallelNode())
			container, err = client.Create(garden.ContainerSpec{
				Network: networkSpec,
			})
			Expect(err).NotTo(HaveOccurred())
			contIfaceName = ethInterfaceName(container)

			networkBridgeName, err = container.Property("kawasaki.bridge-interface")
			Expect(err).NotTo(HaveOccurred())
		})

		var itCleansUpPerContainerNetworkingResources = func() {
			It("should remove iptable entries", func() {
				out, err := runIPTables("-S", "-t", "filter")
				Expect(err).NotTo(HaveOccurred())
				Expect(string(out)).NotTo(MatchRegexp("w-%d-instance.* 177.100.%d.0/24", GinkgoParallelNode(), GinkgoParallelNode()))
			})

			It("should remove virtual ethernet cards", func() {
				ifconfigExits := func() int {
					session, err := gexec.Start(exec.Command("ifconfig", contIfaceName), GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					return session.Wait().ExitCode()
				}
				Eventually(ifconfigExits).ShouldNot(Equal(0))
			})
		}

		var itRemovesTheNetworkBridge = func() {
			It("should remove the network bridge", func() {
				session, err := gexec.Start(
					exec.Command("ip", "link", "show", networkBridgeName),
					GinkgoWriter, GinkgoWriter,
				)
				Expect(err).NotTo(HaveOccurred())

				session.Wait()
				Expect(session.ExitCode()).NotTo(Equal(0))
			})
		}

		Context("when destroy is called", func() {
			JustBeforeEach(func() {
				Expect(client.Destroy(container.Handle())).To(Succeed())
			})

			itCleansUpPerContainerNetworkingResources()
			itRemovesTheNetworkBridge()

			Context("and there was more than one containers in the same subnet", func() {
				var otherContainer garden.Container

				JustBeforeEach(func() {
					var err error

					otherContainer, err = client.Create(garden.ContainerSpec{
						Network: networkSpec,
					})
					Expect(err).NotTo(HaveOccurred())
				})

				JustBeforeEach(func() {
					Expect(client.Destroy(otherContainer.Handle())).To(Succeed())
				})

				itRemovesTheNetworkBridge()
			})
		})
	})

	Context("when the containerd socket has been passed", func() {
		var (
			containerdSession *gexec.Session
			container         garden.Container
		)

		BeforeEach(func() {
			containerdSession = containerdrunner.NewDefaultSession(containerdConfig)
			config.ContainerdSocket = containerdConfig.GRPC.Address
		})

		AfterEach(func() {
			Expect(containerdSession.Terminate().Wait()).To(gexec.Exit(0))
		})

		JustBeforeEach(func() {
			var err error
			container, err = client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("removes the container from ctr lookup", func() {
			err := client.Destroy(container.Handle())
			Expect(err).NotTo(HaveOccurred())

			lookupCommand := exec.Command(containerdBinaries.Ctr, "--address", config.ContainerdSocket, "--namespace", "garden", "containers", "list")

			session, err := gexec.Start(lookupCommand, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Consistently(session).ShouldNot(gbytes.Say(container.Handle()))
			Eventually(session).Should(gexec.Exit(0))
		})
	})
})

func ethInterfaceName(container garden.Container) string {
	buffer := gbytes.NewBuffer()
	proc, err := container.Run(
		garden.ProcessSpec{
			Path: "sh",
			Args: []string{"-c", "ifconfig | grep 'Ethernet' | cut -f 1 -d ' '"},
			User: "root",
		},
		garden.ProcessIO{
			Stdout: buffer,
			Stderr: GinkgoWriter,
		},
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(proc.Wait()).To(Equal(0))

	contIfaceName := string(buffer.Contents()) // g3-abc-1

	return contIfaceName[:len(contIfaceName)-2] + "0" // g3-abc-0
}

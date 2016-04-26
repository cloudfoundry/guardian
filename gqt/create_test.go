package gqt_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"

	. "github.com/cloudfoundry-incubator/guardian/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Creating a Container", func() {
	var (
		args      []string
		client    *runner.RunningGarden
		container garden.Container

		initialSockets int
		initialPipes   int
	)

	BeforeEach(func() {
		args = nil
	})

	JustBeforeEach(func() {
		client = startGarden(args...)
		initialSockets = numOpenSockets(client.Pid)
		initialPipes = numPipes(client.Pid)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	Context("when creating fails", func() {
		var bogusPlugin *os.File

		BeforeEach(func() {
			var err error
			bogusPlugin, err = ioutil.TempFile("", "POTATO")
			Expect(err).ToNot(HaveOccurred())

			Expect(bogusPlugin.Close()).To(Succeed())

			args = []string{
				// force it to fail by specifying invalid network plugin
				"--network-plugin", bogusPlugin.Name(),
			}
		})

		AfterEach(func() {
			Expect(os.Remove(bogusPlugin.Name())).To(Succeed())
		})

		It("returns a nice error rather than timing out", func() {
			_, err := client.Create(garden.ContainerSpec{})
			Expect(err).To(MatchError(ContainSubstring("permission denied")))
		})

		It("cleans up the depot directory", func() {
			_, err := client.Create(garden.ContainerSpec{})
			Expect(err).To(HaveOccurred())

			Expect(ioutil.ReadDir(client.DepotDir)).To(BeEmpty())
		})

		It("cleans up the graph", func() {
			// pre-warm cache to avoid test pollution
			_, err := client.Create(garden.ContainerSpec{})
			Expect(err).To(HaveOccurred())

			prev, err := ioutil.ReadDir(filepath.Join(client.GraphPath, "aufs", "mnt"))
			Expect(err).NotTo(HaveOccurred())

			_, err = client.Create(garden.ContainerSpec{})
			Expect(err).To(HaveOccurred())

			Expect(ioutil.ReadDir(filepath.Join(client.GraphPath, "aufs", "mnt"))).To(HaveLen(len(prev)))
		})
	})

	Context("after creating a container without a specified handle", func() {
		var (
			privileged bool

			initProcPid int
		)

		JustBeforeEach(func() {
			var err error
			container, err = client.Create(garden.ContainerSpec{
				Privileged: privileged,
			})
			Expect(err).NotTo(HaveOccurred())

			initProcPid = initProcessPID(container.Handle())
		})

		It("should create a depot subdirectory based on the container handle", func() {
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
			hostNS, err := gexec.Start(exec.Command("ls", "-l", fmt.Sprintf("/proc/1/ns/%s", ns)), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			containerNS, err := gexec.Start(exec.Command("ls", "-l", fmt.Sprintf("/proc/%d/ns/%s", initProcPid, ns)), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(containerNS).Should(gexec.Exit(0))
			Eventually(hostNS).Should(gexec.Exit(0))

			hostFD := strings.Split(string(hostNS.Out.Contents()), ">")[1]
			containerFD := strings.Split(string(containerNS.Out.Contents()), ">")[1]

			Expect(hostFD).NotTo(Equal(containerFD))
		},
			Entry("should place the container in to the NET namespace", "net"),
			Entry("should place the container in to the IPC namespace", "ipc"),
			Entry("should place the container in to the UTS namespace", "uts"),
			Entry("should place the container in to the PID namespace", "pid"),
			Entry("should place the container in to the MNT namespace", "mnt"),
			Entry("should place the container in to the USER namespace", "user"),
		)

		It("should have the proper uid and gid mappings", func() {
			buffer := gbytes.NewBuffer()
			proc, err := container.Run(garden.ProcessSpec{
				Path: "cat",
				Args: []string{"/proc/self/uid_map"},
			}, garden.ProcessIO{
				Stdout: io.MultiWriter(buffer, GinkgoWriter),
				Stderr: GinkgoWriter,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(proc.Wait()).To(Equal(0))

			Eventually(buffer).Should(gbytes.Say(`0\s+4294967294\s+1\n\s+1\s+1\s+4294967293`))
		})

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
		var rootFSPath string

		JustBeforeEach(func() {
			var err error

			rootFSPath, err = ioutil.TempDir("", "test-rootfs")
			Expect(err).NotTo(HaveOccurred())
			command := fmt.Sprintf("cp -rf %s/* %s", os.Getenv("GARDEN_TEST_ROOTFS"), rootFSPath)
			Expect(exec.Command("sh", "-c", command).Run()).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(rootFSPath, "my-file"), []byte("some-content"), 0644)).To(Succeed())
			Expect(os.Mkdir(path.Join(rootFSPath, "somedir"), 0777)).To(Succeed())

			container, err = client.Create(garden.ContainerSpec{
				RootFSPath: rootFSPath,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("provides the containers with the right rootfs", func() {
			Expect(container).To(HaveFile("/my-file"))
		})

		It("isolates the filesystem properly for multiple containers", func() {
			runCommand(container, "touch", []string{"/somedir/created-file"})
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

			container, err = client.Create(garden.ContainerSpec{
				Handle: "another-banana",
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when creating a container fails", func() {
		It("should not leak networking configuration", func() {
			_, err := client.Create(garden.ContainerSpec{
				Network:    fmt.Sprintf("172.250.%d.20/24", GinkgoParallelNode()),
				RootFSPath: "/banana/does/not/exist",
			})
			Expect(err).To(HaveOccurred())

			session, err := gexec.Start(
				exec.Command("ifconfig"),
				GinkgoWriter, GinkgoWriter,
			)
			Expect(err).NotTo(HaveOccurred())
			Consistently(session).ShouldNot(gbytes.Say(fmt.Sprintf("172-250-%d-0", GinkgoParallelNode())))
		})
	})

	Context("when creating a container and specifying CPU limits", func() {
		readFileContent := func(path string) string {
			content, err := ioutil.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())

			return strings.TrimSpace(string(content))
		}

		createContainerWithCpuShares := func(share uint64) (garden.Container, error) {
			limits := garden.Limits{
				CPU: garden.CPULimits{LimitInShares: share},
			}

			container, err := client.Create(garden.ContainerSpec{
				Limits: limits,
			})

			return container, err
		}

		checkCPUSharesInContainer := func(container garden.Container, clientPid int, expected int) {
			cpuset := readFileContent(fmt.Sprintf("/proc/%d/cpuset", clientPid))
			cpuset = strings.TrimLeft(cpuset, "/")

			cpuSharesPath := fmt.Sprintf("%s/cgroups-%d/cpu/%s/%s/cpu.shares", client.Tmpdir,
				GinkgoParallelNode(), cpuset, container.Handle())

			cpuShares := readFileContent(cpuSharesPath)
			Expect(cpuShares).To(Equal(strconv.Itoa(expected)))
		}

		It("should return an error when the cpu shares is invalid", func() {
			_, err := createContainerWithCpuShares(1)

			Expect(err.Error()).To(ContainSubstring("The minimum allowed cpu-shares is 2"))
		})

		It("should use the default value when the cpu share is set to zero", func() {
			container, err := createContainerWithCpuShares(0)
			Expect(err).NotTo(HaveOccurred())
			checkCPUSharesInContainer(container, client.Pid, 1024)
		})

		It("should use the custom cpu shares limit", func() {
			container, err := createContainerWithCpuShares(2)
			Expect(err).NotTo(HaveOccurred())
			checkCPUSharesInContainer(container, client.Pid, 2)
		})
	})
})

func initProcessPID(handle string) int {
	Eventually(fmt.Sprintf("/run/runc/%s/state.json", handle)).Should(BeAnExistingFile())

	state := struct {
		Pid int `json:"init_process_pid"`
	}{}

	Eventually(func() error {
		stateFile, err := os.Open(fmt.Sprintf("/run/runc/%s/state.json", handle))
		Expect(err).NotTo(HaveOccurred())
		defer stateFile.Close()

		// state.json is sometimes empty immediately after creation, so keep
		// trying until it's valid json
		return json.NewDecoder(stateFile).Decode(&state)
	}).Should(Succeed())

	return state.Pid
}

func runCommand(container garden.Container, path string, args []string) {
	proc, err := container.Run(
		garden.ProcessSpec{
			Path: path,
			Args: args,
		},
		ginkgoIO)
	Expect(err).NotTo(HaveOccurred())

	exitCode, err := proc.Wait()
	Expect(err).NotTo(HaveOccurred())
	Expect(exitCode).To(Equal(0))
}

func numOpenSockets(pid int) (num int) {
	sess, err := gexec.Start(exec.Command("sh", "-c", fmt.Sprintf("lsof -p %d | grep sock", pid)), GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))

	return bytes.Count(sess.Out.Contents(), []byte{'\n'})
}

func numPipes(pid int) (num int) {
	sess, err := gexec.Start(exec.Command("sh", "-c", fmt.Sprintf("lsof -p %d | grep pipe", pid)), GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))

	return bytes.Count(sess.Out.Contents(), []byte{'\n'})
}

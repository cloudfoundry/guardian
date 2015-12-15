package gqt_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"

	. "github.com/cloudfoundry-incubator/guardian/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Creating a Container", func() {
	var (
		client    *runner.RunningGarden
		container garden.Container
	)

	Context("after creating a container without a specified handle", func() {
		var initProcPid int

		BeforeEach(func() {
			client = startGarden()

			var err error
			container, err = client.Create(garden.ContainerSpec{})
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
		)

		Describe("destroying the container", func() {
			var process garden.Process

			BeforeEach(func() {
				var err error
				process, err = container.Run(garden.ProcessSpec{
					Path: "/bin/sh",
					Args: []string{
						"-c", "read x",
					},
				}, ginkgoIO)
				Expect(err).NotTo(HaveOccurred())

				Expect(client.Destroy(container.Handle())).To(Succeed())
			})

			It("should kill the containers init process", func() {
				var killExitCode = func() int {
					sess, err := gexec.Start(exec.Command("kill", "-0", fmt.Sprintf("%d", initProcPid)), GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					sess.Wait(1 * time.Second)
					return sess.ExitCode()
				}

				Eventually(killExitCode, "5s").Should(Equal(1))
			})

			It("should destroy the container's depot directory", func() {
				Expect(filepath.Join(client.DepotDir, container.Handle())).NotTo(BeAnExistingFile())
			})
		})
	})

	Context("after creating a container with a specified root filesystem", func() {
		var rootFSPath string

		BeforeEach(func() {
			var err error

			client = startGarden()
			rootFSPath, err = ioutil.TempDir("", "test-rootfs")
			Expect(err).NotTo(HaveOccurred())
			command := fmt.Sprintf("cp -rf %s/* %s", os.Getenv("GARDEN_TEST_ROOTFS"), rootFSPath)
			Expect(exec.Command("sh", "-c", command).Run()).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(rootFSPath, "my-file"), []byte("some-content"), 0644)).To(Succeed())

			container, err = client.Create(garden.ContainerSpec{
				RootFSPath: rootFSPath,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			client.DestroyAndStop()
		})

		It("provides the containers with the right rootfs", func() {
			Expect(container).To(HaveFile("/my-file"))
		})

		It("isolates the filesystem properly for multiple containers", func() {
			runCommand(container, "touch", []string{"/created-file"})
			Expect(container).To(HaveFile("/created-file"))

			container2, err := client.Create(garden.ContainerSpec{
				RootFSPath: rootFSPath,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(container2).To(HaveFile("/my-file"))
			Expect(container2).NotTo(HaveFile("/created-file"))
		})
	})

	Context("after creating a container with a specified handle", func() {
		BeforeEach(func() {
			Skip("skipping because we don't cleanup the networks, unpend after #101501268")
			client = startGarden()

			var mySpec garden.ContainerSpec
			mySpec = garden.ContainerSpec{
				Handle: "containerA",
			}

			var err error
			container, err = client.Create(mySpec)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			client.DestroyAndStop()
		})

		It("should lookup the right container for the handle", func() {
			lookupContainer, lookupError := client.Lookup("containerA")

			Expect(lookupError).NotTo(HaveOccurred())
			Expect(lookupContainer).To(Equal(container))
		})

		It("allow the container to be created with the same name after destroying", func() {
			client.Destroy(container.Handle())

			var err error
			container, err = client.Create(garden.ContainerSpec{
				Handle: "containerA",
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

func initProcessPID(handle string) int {
	Eventually(fmt.Sprintf("/run/opencontainer/containers/%s/state.json", handle)).Should(BeAnExistingFile())
	stateFile, err := os.Open(fmt.Sprintf("/run/opencontainer/containers/%s/state.json", handle))
	Expect(err).NotTo(HaveOccurred())

	state := struct {
		Pid int `json:"init_process_pid"`
	}{}

	Eventually(func() error {
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

package gqt_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"

	"io"

	. "github.com/cloudfoundry-incubator/guardian/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Creating a Container", func() {
	var (
		client    *runner.RunningGarden
		container garden.Container
	)

	BeforeEach(func() {
		client = startGarden()
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
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

		BeforeEach(func() {
			var err error

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

	Context("when creating a container with RW bind mount", func() {
		var dir string
		BeforeEach(func() {
			var err error
			dir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			ioutil.WriteFile(filepath.Join(dir, "foo"), []byte{}, 755)
		})

		It("should be able to access the foo file", func() {
			container, err := client.Create(garden.ContainerSpec{
				BindMounts: []garden.BindMount{
					garden.BindMount{
						SrcPath: dir,
						DstPath: "/home/alice/some/dir/",
						Mode:    garden.BindMountModeRW,
						Origin:  garden.BindMountOriginHost,
					},
				},
			})

			Expect(err).NotTo(HaveOccurred())

			out := gbytes.NewBuffer()
			proc, err := container.Run(
				garden.ProcessSpec{
					Path: "ls",
					Args: []string{"-l", "/home/alice/some/dir/"},
				},
				garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, out),
					Stderr: io.MultiWriter(GinkgoWriter, out),
				})
			Expect(err).NotTo(HaveOccurred())

			exitCode, err := proc.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(0))

			Expect(out).To(gbytes.Say("foo"))
		})
	})

	Context("when creating a container with RO bind mount", func() {
		var dir string
		BeforeEach(func() {
			var err error
			dir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			ioutil.WriteFile(filepath.Join(dir, "foo"), []byte{}, 755)
		})

		It("should be able to access the foo file", func() {
			container, err := client.Create(garden.ContainerSpec{
				BindMounts: []garden.BindMount{
					garden.BindMount{
						SrcPath: dir,
						DstPath: "/home/alice/some/dir/",
						Mode:    garden.BindMountModeRO,
						Origin:  garden.BindMountOriginHost,
					},
				},
			})

			Expect(err).NotTo(HaveOccurred())

			out := gbytes.NewBuffer()
			proc, err := container.Run(
				garden.ProcessSpec{
					Path: "ls",
					Args: []string{"-l", "/home/alice/some/dir/"},
				},
				garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, out),
					Stderr: io.MultiWriter(GinkgoWriter, out),
				})
			Expect(err).NotTo(HaveOccurred())

			exitCode, err := proc.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(0))

			Expect(out).To(gbytes.Say("foo"))

			proc, err = container.Run(
				garden.ProcessSpec{
					Path: "touch",
					Args: []string{"/home/alice/some/dir/test.txt"},
				},
				garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, out),
					Stderr: io.MultiWriter(GinkgoWriter, out),
				})
			Expect(err).NotTo(HaveOccurred())

			exitCode, _ = proc.Wait()
			Expect(exitCode).ToNot(Equal(0))

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

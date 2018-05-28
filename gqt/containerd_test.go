package gqt_test

import (
	"io/ioutil"
	"os/exec"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/containerdrunner"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Containerd", func() {

	var (
		client            *runner.RunningGarden
		containerdSession *gexec.Session
	)

	BeforeEach(func() {
		runDir, err := ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		containerdConfig := containerdrunner.ContainerdConfig(runDir)
		containerdSession = containerdrunner.NewSession(runDir, containerdBinaries, containerdConfig)

		config.ContainerdSocket = containerdConfig.GRPC.Address
		client = runner.Start(config)
	})

	AfterEach(func() {
		Expect(containerdSession.Terminate().Wait()).To(gexec.Exit(0))
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	Describe("creating containers", func() {
		It("creates a containerd container with running init task", func() {
			container, err := client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			lookupCommand := exec.Command(containerdBinaries.Ctr, "--address", config.ContainerdSocket, "--namespace", "garden", "tasks", "ps", container.Handle())

			session, err := gexec.Start(lookupCommand, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gbytes.Say(container.Handle()))
			Eventually(session).Should(gexec.Exit(0))
		})

		Context("when creating more containers than the maxkeyring limit", func() {
			BeforeEach(func() {
				Expect(ioutil.WriteFile("/proc/sys/kernel/keys/maxkeys", []byte("1"), 0644)).To(Succeed())
			})

			AfterEach(func() {
				Expect(ioutil.WriteFile("/proc/sys/kernel/keys/maxkeys", []byte("200"), 0644)).To(Succeed())
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

	})

	Describe("destroying a container", func() {
		var (
			container garden.Container
		)

		BeforeEach(func() {
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

	Describe("running a process in a container", func() {
		var (
			processID string
			container garden.Container
		)

		BeforeEach(func() {
			var err error
			container, err = client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(client.Destroy(container.Handle())).To(Succeed())
			Expect(containerdSession.Terminate().Wait()).To(gexec.Exit(0))
		})

		It("succeeds", func() {
			process, err := container.Run(garden.ProcessSpec{
				Path: "/bin/sh",
				Args: []string{"-c", "exit 17"},
			}, garden.ProcessIO{})
			Expect(err).NotTo(HaveOccurred())
			statusCode, err := process.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(statusCode).To(Equal(17))
		})

		It("can attach to a process", func() {
			process, err := container.Run(garden.ProcessSpec{
				Path: "/bin/sh",
				Args: []string{"-c", "exit 13"},
			}, garden.ProcessIO{})
			Expect(err).NotTo(HaveOccurred())
			processID = process.ID()

			attachedProcess, err := container.Attach(processID, garden.ProcessIO{})
			Expect(err).NotTo(HaveOccurred())

			exitCode, err := attachedProcess.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(13))
		})
	})
})

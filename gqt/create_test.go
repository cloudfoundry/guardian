package gqt_test

import (
	"os/exec"
	"path/filepath"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Creating a Container", func() {
	var client *runner.RunningGarden

	Context("after creating a container", func() {
		BeforeEach(func() {
			client = startGarden()
			_, err := client.Create(garden.ContainerSpec{
				Handle: "fred",
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create a depot subdirectory based on the container handle", func() {
			Expect(filepath.Join(client.DepotDir, "fred")).To(BeADirectory())
		})

		Describe("created container directories", func() {
			It("should have a config.json", func() {
				Expect(filepath.Join(client.DepotDir, "fred", "config.json")).To(BeARegularFile())
			})

			It("should support creating OCI container manually", func() {
				cmd := exec.Command(OciRuntimeBin)
				cmd.Dir = filepath.Join(client.DepotDir, "fred")

				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gbytes.Say("Pid 1 Running"))
			})
		})
	})
})

package gqt_test

import (
	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Logging", func() {

	Context("when a container is created", func() {
		var (
			client *runner.RunningGarden
		)

		BeforeEach(func() {
			binPath, err := gexec.Build("github.com/cloudfoundry-incubator/guardian/gqt/cmd/runc")
			Expect(err).NotTo(HaveOccurred())

			containerSpec := garden.ContainerSpec{
				RootFSPath: runner.RootFSPath,
			}

			client = startGarden("--runc-bin", binPath)
			_, err = client.Create(containerSpec)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			Expect(client.DestroyAndStop()).To(Succeed())
			gexec.CleanupBuildArtifacts()
		})

		It("forwards RunC logs", func() {
			Eventually(client, "1s").Should(gbytes.Say("guardian-runc-logging-test-info"))
			Eventually(client, "1s").Should(gbytes.Say("guardian-runc-logging-test-warn"))
			Eventually(client, "1s").Should(gbytes.Say("guardian-runc-logging-test-error"))
			Eventually(client, "1s").Should(gbytes.Say("guardian-runc-logging-test-print"))
		})
	})
})

package gqt_test

import (
	"path/filepath"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		})
	})
})

func startGarden(argv ...string) *runner.RunningGarden {
	gardenBin, err := gexec.Build("github.com/cloudfoundry-incubator/guardian/cmd/guardian")
	Expect(err).NotTo(HaveOccurred())

	return runner.Start(gardenBin, argv...)
}

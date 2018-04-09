package gqt_test

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const threePointSevenMegabytes = 3879731

var _ = Describe("when forced to use shed", func() {
	var (
		client *runner.RunningGarden
		limits = garden.Limits{
			Disk: garden.DiskLimits{
				ByteSoft: threePointSevenMegabytes,
				ByteHard: threePointSevenMegabytes,
			},
		}
	)

	BeforeEach(func() {
		config = resetImagePluginConfig()
		client = runner.Start(config)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	It("enforces quotas on cached layers", func() {
		containerSpec := garden.ContainerSpec{
			Image:  garden.ImageRef{URI: "docker:///cfgarden/tutu"},
			Limits: limits,
		}
		_, err := client.Create(containerSpec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("layer size exceeds image quota"))
		_, err = client.Create(containerSpec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("layer size exceeds image quota"))
	})

	It("enforces quotas on decompressed image size", func() {
		containerSpec := garden.ContainerSpec{
			Image:  garden.ImageRef{URI: "docker:///cfgarden/iamthebomb#v2"},
			Limits: limits,
		}

		_, err := client.Create(containerSpec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unexpected EOF"))
		Expect(err.Error()).To(ContainSubstring("layer size exceeds image quota"))
	})
})

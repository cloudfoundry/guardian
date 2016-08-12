package groot_test

import (
	"path"

	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Create with remote images", func() {
	var imageURL string

	BeforeEach(func() {
		imageURL = "docker:///cfgarden/empty:v0.1.0"
	})

	It("creates a root filesystem based on the image provided", func() {
		bundle := integration.CreateBundle(GrootFSBin, StorePath, imageURL, "random-id")

		Expect(path.Join(bundle.RootFSPath(), "hello")).To(BeARegularFile())
	})
})

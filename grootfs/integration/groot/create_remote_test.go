package groot_test

import (
	"archive/tar"
	"os"
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

	Describe("OCI image caching", func() {
		It("caches the image in the store", func() {
			integration.CreateBundle(GrootFSBin, StorePath, imageURL, "random-id")

			blobPath := path.Join(
				StorePath, "cache", "blobs",
				"sha256-6c1f4533b125f8f825188c4f4ff633a338cfce0db2813124d3d518028baf7d7a",
			)
			Expect(blobPath).To(BeARegularFile())
		})

		It("uses the cached image from the store", func() {
			integration.CreateBundle(GrootFSBin, StorePath, imageURL, "random-id")

			// change the cache
			blobPath := path.Join(
				StorePath, "cache", "blobs",
				"sha256-6c1f4533b125f8f825188c4f4ff633a338cfce0db2813124d3d518028baf7d7a",
			)

			blob, err := os.OpenFile(blobPath, os.O_WRONLY, 0666)
			Expect(err).NotTo(HaveOccurred())
			tarWriter := tar.NewWriter(blob)
			Expect(tarWriter.WriteHeader(&tar.Header{
				Name: "i-hacked-your-cache",
				Mode: 0666,
				Size: int64(len([]byte("cache-hit!"))),
			})).To(Succeed())
			_, err = tarWriter.Write([]byte("cache-hit!"))
			Expect(err).NotTo(HaveOccurred())
			Expect(tarWriter.Close()).To(Succeed())

			bundle := integration.CreateBundle(GrootFSBin, StorePath, imageURL, "random-id-2")
			Expect(path.Join(bundle.RootFSPath(), "i-hacked-your-cache")).To(BeARegularFile())
		})
	})
})

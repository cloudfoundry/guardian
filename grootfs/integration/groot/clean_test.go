package groot_test

import (
	"io/ioutil"
	"path"

	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/store"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Clean", func() {
	BeforeEach(func() {
		integration.CreateBundle(GrootFSBin, StorePath, DraxBin, "docker:///cfgarden/empty:v0.1.1", "my-bundle-1", 0)
	})

	AfterEach(func() {
		integration.DeleteBundle(GrootFSBin, StorePath, DraxBin, "my-bundle-1")
	})

	Context("when cleaning up volumes", func() {
		BeforeEach(func() {
			integration.CreateBundle(GrootFSBin, StorePath, DraxBin, "docker:///busybox", "my-bundle-2", 0)
		})

		JustBeforeEach(func() {
			integration.DeleteBundle(GrootFSBin, StorePath, DraxBin, "my-bundle-2")
		})

		It("removes volumes that are not currently linked to bundles", func() {
			contents, err := ioutil.ReadDir(path.Join(StorePath, store.VOLUMES_DIR_NAME))
			Expect(err).NotTo(HaveOccurred())
			Expect(len(contents)).To(Equal(3))

			Expect(Groot.Clean()).To(Succeed())

			contents, err = ioutil.ReadDir(path.Join(StorePath, store.VOLUMES_DIR_NAME))
			Expect(err).NotTo(HaveOccurred())
			Expect(len(contents)).To(Equal(2))
			Expect(path.Join(StorePath, "volumes", "sha256:afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5")).To(BeADirectory())
			Expect(path.Join(StorePath, "volumes", "sha256:9242945d3c9c7cf5f127f9352fea38b1d3efe62ee76e25f70a3e6db63a14c233")).To(BeADirectory())
		})
	})

	Context("when cleaning up blobs from cache", func() {
		It("removes the blobs", func() {
			contents, err := ioutil.ReadDir(path.Join(StorePath, store.CACHE_DIR_NAME, "blobs"))
			Expect(err).NotTo(HaveOccurred())
			Expect(len(contents)).To(BeNumerically(">", 0))

			Expect(Groot.Clean()).To(Succeed())

			contents, err = ioutil.ReadDir(path.Join(StorePath, store.CACHE_DIR_NAME, "blobs"))
			Expect(err).NotTo(HaveOccurred())
			Expect(len(contents)).To(Equal(0))
		})
	})
})

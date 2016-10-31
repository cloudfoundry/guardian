package groot_test

import (
	"io/ioutil"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Clean", func() {
	BeforeEach(func() {
		_, err := Runner.Create(groot.CreateSpec{
			ID:    "my-bundle-1",
			Image: "docker:///cfgarden/empty:v0.1.1",
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(Runner.Delete("my-bundle-1")).To(Succeed())
	})

	It("removes the cached blobs", func() {
		preContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.CACHE_DIR_NAME, "blobs"))
		Expect(err).NotTo(HaveOccurred())
		Expect(len(preContents)).To(BeNumerically(">", 0))

		_, err = Runner.Clean(0)
		Expect(err).NotTo(HaveOccurred())

		afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.CACHE_DIR_NAME, "blobs"))
		Expect(err).NotTo(HaveOccurred())
		Expect(afterContents).To(HaveLen(0))
	})

	Context("when there are unused layers", func() {
		BeforeEach(func() {
			_, err := Runner.Create(groot.CreateSpec{
				ID:    "my-bundle-2",
				Image: "docker:///busybox",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(Runner.Delete("my-bundle-2")).To(Succeed())
		})

		It("removes unused volumes", func() {
			preContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
			Expect(err).NotTo(HaveOccurred())
			Expect(preContents).To(HaveLen(3))

			_, err = Runner.Clean(0)
			Expect(err).NotTo(HaveOccurred())

			afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
			Expect(err).NotTo(HaveOccurred())
			Expect(afterContents).To(HaveLen(2))
			for _, layer := range testhelpers.EmptyImageV011.Layers {
				Expect(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME, layer.ChainID)).To(BeADirectory())
			}
		})

		Context("and a threshold is set", func() {
			var cleanupThresholdInBytes uint64

			Context("and the total is less than the threshold", func() {
				BeforeEach(func() {
					// 688128      # Blob cache
					// 16384       # Empty layer
					// 16384       # Empty layer
					// 16384       # Empty rootfs
					// 1179648     # Busybox layer
					// ----------------------------------
					// = 1916928 ~= 1.83 MB

					cleanupThresholdInBytes = 2500000
				})

				It("does not remove the cached blobs", func() {
					preContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.CACHE_DIR_NAME, "blobs"))
					Expect(err).NotTo(HaveOccurred())

					_, err = Runner.Clean(cleanupThresholdInBytes)
					Expect(err).NotTo(HaveOccurred())

					afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.CACHE_DIR_NAME, "blobs"))
					Expect(err).NotTo(HaveOccurred())
					Expect(afterContents).To(HaveLen(len(preContents)))
				})

				It("does not remove the unused volumes", func() {
					preContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
					Expect(err).NotTo(HaveOccurred())

					_, err = Runner.Clean(cleanupThresholdInBytes)
					Expect(err).NotTo(HaveOccurred())

					afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
					Expect(err).NotTo(HaveOccurred())
					Expect(afterContents).To(HaveLen(len(preContents)))
				})

				It("reports that it was a no-op", func() {
					output, err := Runner.Clean(cleanupThresholdInBytes)
					Expect(err).NotTo(HaveOccurred())
					Expect(output).To(ContainSubstring("threshold not reached: skipping clean"))
				})
			})

			Context("and the total is more than the threshold", func() {
				BeforeEach(func() {
					cleanupThresholdInBytes = 70000
				})

				It("removes the cached blobs", func() {
					preContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.CACHE_DIR_NAME, "blobs"))
					Expect(err).NotTo(HaveOccurred())
					Expect(preContents).To(HaveLen(5))

					_, err = Runner.Clean(cleanupThresholdInBytes)
					Expect(err).NotTo(HaveOccurred())

					afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.CACHE_DIR_NAME, "blobs"))
					Expect(err).NotTo(HaveOccurred())
					Expect(afterContents).To(HaveLen(0))
				})

				It("removes the unused volumes", func() {
					preContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
					Expect(err).NotTo(HaveOccurred())
					Expect(preContents).To(HaveLen(3))

					_, err = Runner.Clean(cleanupThresholdInBytes)
					Expect(err).NotTo(HaveOccurred())

					afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
					Expect(err).NotTo(HaveOccurred())
					Expect(afterContents).To(HaveLen(2))
					for _, layer := range testhelpers.EmptyImageV011.Layers {
						Expect(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME, layer.ChainID)).To(BeADirectory())
					}
				})
			})
		})
	})
})

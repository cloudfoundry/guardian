package store_test

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Configurer", func() {
	var (
		storePath string

		logger     lager.Logger
		configurer *store.Configurer
	)

	BeforeEach(func() {
		tempDir, err := ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		storePath = path.Join(tempDir, "store")

		logger = lagertest.NewTestLogger("store-configurer")
		configurer = store.NewConfigurer()
	})

	AfterEach(func() {
		Expect(os.RemoveAll(path.Dir(storePath))).To(Succeed())
	})

	Describe("Ensure", func() {
		It("should create the store directory", func() {
			Expect(configurer.Ensure(logger, storePath)).To(Succeed())

			Expect(storePath).To(BeADirectory())
		})

		It("should create the correct internal structure", func() {
			Expect(configurer.Ensure(logger, storePath)).To(Succeed())

			Expect(filepath.Join(storePath, "bundles")).To(BeADirectory())
			Expect(filepath.Join(storePath, "cache")).To(BeADirectory())
			Expect(filepath.Join(storePath, "cache", "blobs")).To(BeADirectory())
			Expect(filepath.Join(storePath, "volumes")).To(BeADirectory())
		})

		Context("when the base directory does not exist", func() {
			It("should return an error", func() {
				Expect(configurer.Ensure(logger, "/not/exist")).To(
					MatchError(ContainSubstring("making directory")),
				)
			})
		})

		Context("when the store already exists", func() {
			It("should succeed", func() {
				Expect(os.Mkdir(storePath, 0700)).To(Succeed())
				Expect(configurer.Ensure(logger, storePath)).To(Succeed())
			})

			Context("and it's a regular file", func() {
				It("should return an error", func() {
					Expect(ioutil.WriteFile(storePath, []byte("hello"), 0600)).To(Succeed())

					Expect(configurer.Ensure(logger, storePath)).To(
						MatchError(ContainSubstring("is not a directory")),
					)
				})
			})
		})

		Context("when any internal directory already exists", func() {
			It("should succeed", func() {
				Expect(os.MkdirAll(filepath.Join(storePath, "volumes"), 0700)).To(Succeed())
				Expect(configurer.Ensure(logger, storePath)).To(Succeed())
			})

			Context("and it's a regular file", func() {
				It("should return an error", func() {
					Expect(os.Mkdir(storePath, 0700)).To(Succeed())
					Expect(ioutil.WriteFile(filepath.Join(storePath, "volumes"), []byte("hello"), 0600)).To(Succeed())

					Expect(configurer.Ensure(logger, storePath)).To(
						MatchError(ContainSubstring("is not a directory")),
					)
				})
			})
		})
	})
})

package store_test

import (
	"io/ioutil"
	"os"
	"path"

	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Store", func() {
	var (
		logger lager.Logger

		storePath string

		grph *store.Store
	)

	BeforeEach(func() {
		var err error

		storePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		logger = lagertest.NewTestLogger("test-store")
		grph = store.NewStore(storePath)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
	})

	Describe("MakeBundle", func() {
		It("should return a bundle directory", func() {
			bundle, err := grph.MakeBundle(logger, "some-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(bundle.Path()).To(BeADirectory())
		})

		It("should keep the bundles in the same bundle directory", func() {
			someBundle, err := grph.MakeBundle(logger, "some-id")
			Expect(err).NotTo(HaveOccurred())
			anotherBundle, err := grph.MakeBundle(logger, "another-id")
			Expect(err).NotTo(HaveOccurred())

			Expect(someBundle.Path()).NotTo(BeEmpty())
			Expect(anotherBundle.Path()).NotTo(BeEmpty())

			bundles, err := ioutil.ReadDir(path.Join(storePath, store.BUNDLES_DIR_NAME))
			Expect(err).NotTo(HaveOccurred())
			Expect(len(bundles)).To(Equal(2))
		})

		Context("when calling it with two different ids", func() {
			It("should return two different bundle paths", func() {
				bundle, err := grph.MakeBundle(logger, "some-id")
				Expect(err).NotTo(HaveOccurred())

				anotherBundle, err := grph.MakeBundle(logger, "another-id")
				Expect(err).NotTo(HaveOccurred())

				Expect(bundle.Path()).NotTo(Equal(anotherBundle.Path()))
			})
		})

		Context("when using the same id twice", func() {
			It("should return an error", func() {
				_, err := grph.MakeBundle(logger, "some-id")
				Expect(err).NotTo(HaveOccurred())

				_, err = grph.MakeBundle(logger, "some-id")
				Expect(err).To(MatchError("bundle for id `some-id` already exists"))
			})
		})

		Context("when the store path does not exist", func() {
			BeforeEach(func() {
				storePath = "/non/existing/store"
			})

			It("should return an error", func() {
				_, err := grph.MakeBundle(logger, "some-id")
				Expect(err).To(MatchError(ContainSubstring("making bundle path")))
			})
		})
	})

	Describe("DeleteBundle", func() {
		var bundlePath string

		BeforeEach(func() {
			bundlePath = path.Join(storePath, store.BUNDLES_DIR_NAME, "some-id")
			Expect(os.MkdirAll(bundlePath, 0755)).To(Succeed())
			Expect(ioutil.WriteFile(path.Join(bundlePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())
		})

		It("deletes an existing bundle", func() {
			Expect(grph.DeleteBundle(logger, "some-id")).To(Succeed())
			Expect(bundlePath).NotTo(BeAnExistingFile())
		})

		Context("when bundle does not exist", func() {
			It("returns an error", func() {
				err := grph.DeleteBundle(logger, "cake")
				Expect(err).To(MatchError(ContainSubstring("bundle path not found")))
			})
		})

		Context("when deleting the folder fails", func() {
			BeforeEach(func() {
				Expect(os.Chmod(bundlePath, 0666)).To(Succeed())
			})

			AfterEach(func() {
				// we need to revert permissions because of the outer AfterEach
				Expect(os.Chmod(bundlePath, 0755)).To(Succeed())
			})

			It("returns an error", func() {
				err := grph.DeleteBundle(logger, "some-id")
				Expect(err).To(MatchError(ContainSubstring("deleting bundle path")))
			})
		})
	})
})

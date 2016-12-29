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
		It("creates the store directory", func() {
			Expect(configurer.Ensure(logger, storePath)).To(Succeed())

			Expect(storePath).To(BeADirectory())
		})

		It("creates the correct internal structure", func() {
			Expect(configurer.Ensure(logger, storePath)).To(Succeed())

			Expect(filepath.Join(storePath, "images")).To(BeADirectory())
			Expect(filepath.Join(storePath, "cache")).To(BeADirectory())
			Expect(filepath.Join(storePath, "volumes")).To(BeADirectory())
			Expect(filepath.Join(storePath, "locks")).To(BeADirectory())
			Expect(filepath.Join(storePath, "meta")).To(BeADirectory())
			Expect(filepath.Join(storePath, "meta", "dependencies")).To(BeADirectory())
		})

		It("chmods the storePath to 700", func() {
			Expect(configurer.Ensure(logger, storePath)).To(Succeed())

			stat, err := os.Stat(storePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0700)))
		})

		It("doesn't fail on race conditions", func() {
			for i := 0; i < 50; i++ {
				storePath, err := ioutil.TempDir("", "")
				Expect(err).NotTo(HaveOccurred())
				start1 := make(chan bool)
				start2 := make(chan bool)

				go func() {
					defer GinkgoRecover()
					<-start1
					Expect(configurer.Ensure(logger, storePath)).To(Succeed())
					close(start1)
				}()

				go func() {
					defer GinkgoRecover()
					<-start2
					Expect(configurer.Ensure(logger, storePath)).To(Succeed())
					close(start2)
				}()

				start1 <- true
				start2 <- true

				Eventually(start1).Should(BeClosed())
				Eventually(start2).Should(BeClosed())
			}
		})

		Context("when the base directory does not exist", func() {
			It("returns an error", func() {
				Expect(configurer.Ensure(logger, "/not/exist")).To(
					MatchError(ContainSubstring("making directory")),
				)
			})
		})

		Context("when the store already exists", func() {
			It("succeeds", func() {
				Expect(os.Mkdir(storePath, 0700)).To(Succeed())
				Expect(configurer.Ensure(logger, storePath)).To(Succeed())
			})

			Context("and it's a regular file", func() {
				It("returns an error", func() {
					Expect(ioutil.WriteFile(storePath, []byte("hello"), 0600)).To(Succeed())

					Expect(configurer.Ensure(logger, storePath)).To(
						MatchError(ContainSubstring("is not a directory")),
					)
				})
			})
		})

		Context("when any internal directory already exists", func() {
			It("succeeds", func() {
				Expect(os.MkdirAll(filepath.Join(storePath, "volumes"), 0700)).To(Succeed())
				Expect(configurer.Ensure(logger, storePath)).To(Succeed())
			})

			Context("and it's a regular file", func() {
				It("returns an error", func() {
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

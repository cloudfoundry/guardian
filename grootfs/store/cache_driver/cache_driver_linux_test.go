package cache_driver_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/fetcher"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/cache_driver"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("CacheDriver", func() {
	var (
		cacheDriver *cache_driver.CacheDriver
		storePath   string
		cachePath   string

		logger            *lagertest.TestLogger
		blobFuncCallCount int
		blobFunc          fetcher.RemoteBlobFunc
	)

	BeforeEach(func() {
		var err error
		storePath, err = ioutil.TempDir("", "store")
		Expect(err).ToNot(HaveOccurred())
		cachePath = filepath.Join(storePath, "cache")
		Expect(os.MkdirAll(cachePath, 0755)).To(Succeed())

		logger = lagertest.NewTestLogger("cacheDriver")
		cacheDriver = cache_driver.NewCacheDriver(storePath)

		blobFuncCallCount = 0
		blobFunc = func(logger lager.Logger) ([]byte, int64, error) {
			blobFuncCallCount += 1
			return []byte("hello world"), 0, nil
		}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
	})

	Describe("FetchBlob", func() {
		Context("when the blob is not cached", func() {
			It("calls the blobFunc function", func() {
				_, _, err := cacheDriver.FetchBlob(logger, "my-blob", blobFunc)
				Expect(err).ToNot(HaveOccurred())
				Expect(blobFuncCallCount).To(Equal(1))
			})

			It("returns the content returned by blobFunc", func() {
				contents, _, err := cacheDriver.FetchBlob(logger, "my-blob", blobFunc)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(contents)).To(Equal("hello world"))
			})

			It("returns the correct blob size", func() {
				blobFunc = func(_ lager.Logger) ([]byte, int64, error) {
					buffer := gbytes.NewBuffer()
					buffer.Write([]byte("hello world"))
					return buffer.Contents(), 1024, nil
				}

				_, size, err := cacheDriver.FetchBlob(logger, "my-blob", blobFunc)
				Expect(err).ToNot(HaveOccurred())
				Expect(size).To(Equal(int64(1024)))
			})

			Context("when the store does not exist", func() {
				BeforeEach(func() {
					cacheDriver = cache_driver.NewCacheDriver("/non/existing/store")
				})

				It("returns an error", func() {
					_, _, err := cacheDriver.FetchBlob(logger, "my-blob", blobFunc)
					Expect(err).To(MatchError(ContainSubstring("creating cached blob file")))
				})
			})

			It("stores the blob returned by blobFunc in the cache", func() {
				_, _, err := cacheDriver.FetchBlob(logger, "my-blob", blobFunc)
				Expect(err).ToNot(HaveOccurred())

				theBlobPath := blobPath(storePath, "my-blob")
				Expect(theBlobPath).To(BeARegularFile())

				cachedBlobContents, err := ioutil.ReadFile(theBlobPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(cachedBlobContents)).To(Equal("hello world"))
			})

			Context("when blobFunc fails", func() {
				BeforeEach(func() {
					blobFunc = func(logger lager.Logger) ([]byte, int64, error) {
						return nil, 0, errors.New("failed getting remote stream")
					}
				})

				It("returns the error", func() {
					_, _, err := cacheDriver.FetchBlob(logger, "my-blob", blobFunc)
					Expect(err).To(MatchError(ContainSubstring("failed getting remote stream")))
				})

				It("cleans up corrupted stated", func() {
					_, _, err := cacheDriver.FetchBlob(logger, "my-blob", blobFunc)
					Expect(err).To(MatchError(ContainSubstring("failed getting remote stream")))
				})

				It("cleans up corrupted stated", func() {
					_, _, err := cacheDriver.FetchBlob(logger, "my-blob", blobFunc)
					Expect(err).To(MatchError(ContainSubstring("failed getting remote stream")))
					theBlobPath := blobPath(storePath, "my-blob")
					Expect(theBlobPath).NotTo(BeARegularFile())
				})
			})
		})

		Context("when the blob is cached", func() {
			BeforeEach(func() {
				_, _, err := cacheDriver.FetchBlob(logger, "my-blob", blobFunc)
				Expect(err).ToNot(HaveOccurred())

				// reset the test counter
				blobFuncCallCount = 0
			})

			It("does not call blobFunc", func() {
				_, _, err := cacheDriver.FetchBlob(logger, "my-blob", blobFunc)
				Expect(err).ToNot(HaveOccurred())

				Expect(blobFuncCallCount).To(Equal(0))
			})

			It("returns the correct blob size", func() {
				_, size, err := cacheDriver.FetchBlob(logger, "my-blob", blobFunc)

				Expect(err).ToNot(HaveOccurred())
				Expect(size).To(Equal(int64(len("hello world"))))
			})

			Context("but the cached file is not a file", func() {
				BeforeEach(func() {
					theBlobPath := blobPath(storePath, "my-blob")
					Expect(os.Remove(theBlobPath)).To(Succeed())
					Expect(os.MkdirAll(theBlobPath, 0755)).To(Succeed())
				})

				It("returns an error", func() {
					_, _, err := cacheDriver.FetchBlob(logger, "my-blob", blobFunc)
					Expect(err).To(MatchError(ContainSubstring("exists but it's not a regular file")))
				})
			})
		})
	})

	Describe("Clean", func() {
		BeforeEach(func() {
			Expect(ioutil.WriteFile(path.Join(cachePath, "cached-1"), []byte{}, 0666)).To(Succeed())
			Expect(ioutil.WriteFile(path.Join(cachePath, "cached-2"), []byte{}, 0666)).To(Succeed())
		})

		It("cleans up the cache contents", func() {
			contents, err := ioutil.ReadDir(cachePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(contents)).To(Equal(2))

			Expect(cacheDriver.Clean(logger)).To(Succeed())

			contents, err = ioutil.ReadDir(cachePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(contents)).To(Equal(0))
		})
	})
})

func blobPath(storePath, id string) string {
	return filepath.Join(storePath, store.CACHE_DIR_NAME, id)
}

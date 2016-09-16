package cache_driver_test

import (
	"errors"
	"io/ioutil"
	"os"
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

		logger            *lagertest.TestLogger
		blobFuncCallCount int
		blobFunc          fetcher.RemoteBlobFunc
	)

	BeforeEach(func() {
		var err error
		storePath, err = ioutil.TempDir("", "store")
		Expect(err).ToNot(HaveOccurred())
		Expect(os.MkdirAll(filepath.Join(storePath, "cache", "blobs"), 0755)).To(Succeed())

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

	Context("when the blob is not cached", func() {
		It("calls the blobFunc function", func() {
			_, _, err := cacheDriver.StreamBlob(logger, "my-blob", blobFunc)
			Expect(err).ToNot(HaveOccurred())
			Expect(blobFuncCallCount).To(Equal(1))
		})

		It("returns the content returned by blobFunc", func() {
			blob, _, err := cacheDriver.StreamBlob(logger, "my-blob", blobFunc)
			Expect(err).ToNot(HaveOccurred())

			contents, err := ioutil.ReadAll(blob)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("hello world"))
		})

		It("returns the correct blob size", func() {
			blobFunc = func(_ lager.Logger) ([]byte, int64, error) {
				buffer := gbytes.NewBuffer()
				buffer.Write([]byte("hello world"))
				return buffer.Contents(), 1024, nil
			}

			_, size, err := cacheDriver.StreamBlob(logger, "my-blob", blobFunc)
			Expect(err).ToNot(HaveOccurred())
			Expect(size).To(Equal(int64(1024)))
		})

		Context("when the store does not exist", func() {
			BeforeEach(func() {
				cacheDriver = cache_driver.NewCacheDriver("/non/existing/store")
			})

			It("returns an error", func() {
				_, _, err := cacheDriver.StreamBlob(logger, "my-blob", blobFunc)
				Expect(err).To(MatchError(ContainSubstring("creating cached blob file")))
			})
		})

		It("stores the blob returned by blobFunc in the cache", func() {
			blobContent, _, err := cacheDriver.StreamBlob(logger, "my-blob", blobFunc)
			Expect(err).ToNot(HaveOccurred())

			theBlobPath := blobPath(storePath, "my-blob")
			Expect(theBlobPath).To(BeARegularFile())

			// consume the blob first
			_, err = ioutil.ReadAll(blobContent)
			Expect(err).NotTo(HaveOccurred())

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
				_, _, err := cacheDriver.StreamBlob(logger, "my-blob", blobFunc)
				Expect(err).To(MatchError(ContainSubstring("failed getting remote stream")))
			})

			It("cleans up corrupted stated", func() {
				_, _, err := cacheDriver.StreamBlob(logger, "my-blob", blobFunc)
				Expect(err).To(MatchError(ContainSubstring("failed getting remote stream")))
			})

			It("cleans up corrupted stated", func() {
				_, _, err := cacheDriver.StreamBlob(logger, "my-blob", blobFunc)
				Expect(err).To(MatchError(ContainSubstring("failed getting remote stream")))
				theBlobPath := blobPath(storePath, "my-blob")
				Expect(theBlobPath).NotTo(BeARegularFile())
			})
		})
	})

	Context("when the blob is cached", func() {
		BeforeEach(func() {
			stream, _, err := cacheDriver.StreamBlob(logger, "my-blob", blobFunc)
			Expect(err).ToNot(HaveOccurred())

			// consume the stream first
			_, err = ioutil.ReadAll(stream)
			Expect(err).NotTo(HaveOccurred())

			// reset the test counter
			blobFuncCallCount = 0
		})

		It("does not call blobFunc", func() {
			_, _, err := cacheDriver.StreamBlob(logger, "my-blob", blobFunc)
			Expect(err).ToNot(HaveOccurred())

			Expect(blobFuncCallCount).To(Equal(0))
		})

		It("returns the correct blob size", func() {
			_, size, err := cacheDriver.StreamBlob(logger, "my-blob", blobFunc)

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
				_, _, err := cacheDriver.StreamBlob(logger, "my-blob", blobFunc)
				Expect(err).To(MatchError(ContainSubstring("exists but it's not a regular file")))
			})
		})

		Context("but it does not have access to the cache", func() {
			BeforeEach(func() {
				Expect(os.RemoveAll(filepath.Join(storePath, "cache", "blobs"))).To(Succeed())
				Expect(os.MkdirAll(filepath.Join(storePath, "cache", "blobs"), 0000)).To(Succeed())
			})

			It("returns an error", func() {
				_, _, err := cacheDriver.StreamBlob(logger, "my-blob", blobFunc)
				Expect(err).To(MatchError(ContainSubstring("checking if the blob exists")))
			})
		})

		Context("but it does not have access to the cached blob", func() {
			BeforeEach(func() {
				theBlobPath := blobPath(storePath, "my-blob")
				Expect(os.RemoveAll(theBlobPath)).To(Succeed())
				Expect(ioutil.WriteFile(theBlobPath, []byte("hello world"), 000)).To(Succeed())
			})

			It("returns an error", func() {
				_, _, err := cacheDriver.StreamBlob(logger, "my-blob", blobFunc)
				Expect(err).To(MatchError(ContainSubstring("accessing the cached blob")))
			})
		})
	})
})

func blobPath(storePath, id string) string {
	return filepath.Join(storePath, store.CACHE_DIR_NAME, "blobs", id)
}

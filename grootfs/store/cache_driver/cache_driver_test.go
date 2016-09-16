package cache_driver_test

import (
	"errors"
	"io"
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

		logger              *lagertest.TestLogger
		streamBlobCallCount int
		streamBlob          fetcher.StreamBlob
	)

	BeforeEach(func() {
		var err error
		storePath, err = ioutil.TempDir("", "store")
		Expect(err).ToNot(HaveOccurred())
		Expect(os.MkdirAll(filepath.Join(storePath, "cache", "blobs"), 0755)).To(Succeed())

		logger = lagertest.NewTestLogger("cacheDriver")
		cacheDriver = cache_driver.NewCacheDriver(storePath)

		streamBlobCallCount = 0
		streamBlob = func(logger lager.Logger) (io.ReadCloser, int64, error) {
			streamBlobCallCount += 1

			buffer := gbytes.NewBuffer()
			buffer.Write([]byte("hello world"))
			return buffer, 0, nil
		}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
	})

	Context("when the blob is not cached", func() {
		It("calls the streamBlob function", func() {
			_, _, err := cacheDriver.Blob(logger, "my-blob", streamBlob)
			Expect(err).ToNot(HaveOccurred())
			Expect(streamBlobCallCount).To(Equal(1))
		})

		It("returns the stream returned by streamBlob", func() {
			stream, _, err := cacheDriver.Blob(logger, "my-blob", streamBlob)
			Expect(err).ToNot(HaveOccurred())

			contents, err := ioutil.ReadAll(stream)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("hello world"))
		})

		It("returns the correct blob size", func() {
			streamBlob = func(_ lager.Logger) (io.ReadCloser, int64, error) {
				buffer := gbytes.NewBuffer()
				buffer.Write([]byte("hello world"))
				return buffer, 1024, nil
			}

			_, size, err := cacheDriver.Blob(logger, "my-blob", streamBlob)
			Expect(err).ToNot(HaveOccurred())
			Expect(size).To(Equal(int64(1024)))
		})

		Context("when the store does not exist", func() {
			BeforeEach(func() {
				cacheDriver = cache_driver.NewCacheDriver("/non/existing/store")
			})

			It("returns an error", func() {
				_, _, err := cacheDriver.Blob(logger, "my-blob", streamBlob)
				Expect(err).To(MatchError(ContainSubstring("creating cached blob file")))
			})
		})

		It("stores the stream returned by streamBlob in the cache", func() {
			stream, _, err := cacheDriver.Blob(logger, "my-blob", streamBlob)
			Expect(err).ToNot(HaveOccurred())

			theBlobPath := blobPath(storePath, "my-blob")
			Expect(theBlobPath).To(BeARegularFile())

			// consume the stream first
			_, err = ioutil.ReadAll(stream)
			Expect(err).NotTo(HaveOccurred())

			cachedBlobContents, err := ioutil.ReadFile(theBlobPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(cachedBlobContents)).To(Equal("hello world"))
		})

		Context("when streamBlob fails", func() {
			BeforeEach(func() {
				streamBlob = func(logger lager.Logger) (io.ReadCloser, int64, error) {
					return nil, 0, errors.New("failed getting remote stream")
				}
			})

			It("returns the error", func() {
				_, _, err := cacheDriver.Blob(logger, "my-blob", streamBlob)
				Expect(err).To(MatchError(ContainSubstring("failed getting remote stream")))
			})

			It("cleans up corrupted stated", func() {
				_, _, err := cacheDriver.Blob(logger, "my-blob", streamBlob)
				Expect(err).To(MatchError(ContainSubstring("failed getting remote stream")))
				theBlobPath := blobPath(storePath, "my-blob")
				Expect(theBlobPath).NotTo(BeARegularFile())
			})
		})

		Context("when streamBlob fails to stream all the content", func() {
			var file *os.File

			BeforeEach(func() {
				streamBlob = func(logger lager.Logger) (io.ReadCloser, int64, error) {
					file = os.NewFile(uintptr(100), "my-invalid-file")
					defer file.Close()
					return file, 0, nil
				}
			})

			It("cleans up corrupted stated", func() {
				_, _, err := cacheDriver.Blob(logger, "my-corrupted-blob", streamBlob)
				Expect(err).NotTo(HaveOccurred())
				theBlobPath := blobPath(storePath, "my-corrupted-blob")
				Eventually(theBlobPath).ShouldNot(BeARegularFile())
			})
		})
	})

	Context("when the blob is cached", func() {
		BeforeEach(func() {
			stream, _, err := cacheDriver.Blob(logger, "my-blob", streamBlob)
			Expect(err).ToNot(HaveOccurred())

			// consume the stream first
			_, err = ioutil.ReadAll(stream)
			Expect(err).NotTo(HaveOccurred())

			// reset the test counter
			streamBlobCallCount = 0
		})

		It("does not call streamBlob", func() {
			_, _, err := cacheDriver.Blob(logger, "my-blob", streamBlob)
			Expect(err).ToNot(HaveOccurred())

			Expect(streamBlobCallCount).To(Equal(0))
		})

		It("returns the correct blob size", func() {
			_, size, err := cacheDriver.Blob(logger, "my-blob", streamBlob)

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
				_, _, err := cacheDriver.Blob(logger, "my-blob", streamBlob)
				Expect(err).To(MatchError(ContainSubstring("exists but it's not a regular file")))
			})
		})

		Context("but it does not have access to the cache", func() {
			BeforeEach(func() {
				Expect(os.RemoveAll(filepath.Join(storePath, "cache", "blobs"))).To(Succeed())
				Expect(os.MkdirAll(filepath.Join(storePath, "cache", "blobs"), 0000)).To(Succeed())
			})

			It("returns an error", func() {
				_, _, err := cacheDriver.Blob(logger, "my-blob", streamBlob)
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
				_, _, err := cacheDriver.Blob(logger, "my-blob", streamBlob)
				Expect(err).To(MatchError(ContainSubstring("accessing the cached blob")))
			})
		})
	})
})

func blobPath(storePath, id string) string {
	return filepath.Join(storePath, store.CACHE_DIR_NAME, "blobs", id)
}

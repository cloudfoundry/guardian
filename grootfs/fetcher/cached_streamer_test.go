package fetcher_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/cloner/clonerfakes"
	"code.cloudfoundry.org/grootfs/fetcher"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("CachedStreamer", func() {

	var (
		cacheDir         string
		logger           *lagertest.TestLogger
		streamer         *fetcher.CachedStreamer
		internalStreamer *clonerfakes.FakeStreamer
	)

	BeforeEach(func() {
		var err error
		logger = lagertest.NewTestLogger("Streamer")
		cacheDir, err = ioutil.TempDir("", "streamer-cache")
		Expect(err).NotTo(HaveOccurred())

		internalStreamer = new(clonerfakes.FakeStreamer)
		streamer = fetcher.NewCachedStreamer(cacheDir, internalStreamer)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(cacheDir)
	})

	Describe("Stream", func() {
		BeforeEach(func() {
			buffer := gbytes.NewBuffer()
			buffer.Write([]byte("hello world"))

			internalStreamer.StreamReturns(buffer, 0, nil)
		})

		Context("when the object is not cached", func() {
			It("returns a correct reader to the object", func() {
				reader, _, err := streamer.Stream(logger, "hello")
				Expect(err).ToNot(HaveOccurred())

				readerContent := make([]byte, 11)
				reader.Read(readerContent)
				Expect(readerContent).To(Equal([]byte("hello world")))
			})

			It("caches the object properly", func() {
				cachedBlobPath := filepath.Join(cacheDir, "hello")
				Expect(cachedBlobPath).ToNot(BeAnExistingFile())

				_, _, err := streamer.Stream(logger, "hello")
				Expect(err).ToNot(HaveOccurred())

				Expect(cachedBlobPath).To(BeAnExistingFile())
				cachedContent, err := ioutil.ReadFile(cachedBlobPath)
				Expect(err).ToNot(HaveOccurred())

				Expect(cachedContent).To(Equal([]byte("hello world")))
			})

			Context("when creating the cache fails", func() {
				It("returns an error", func() {
					os.RemoveAll(cacheDir)
					_, _, err := streamer.Stream(logger, "hello")
					Expect(err).To(MatchError(ContainSubstring("creating cache:")))
				})
			})

			Context("when reading from the internal streamer fails", func() {
				BeforeEach(func() {
					internalStreamer.StreamReturns(nil, 0, errors.New("internal streamer exploded"))
				})

				It("returns an error", func() {
					_, _, err := streamer.Stream(logger, "hello")
					Expect(err).To(MatchError(ContainSubstring("internal streamer exploded")))
				})
			})
		})

		Context("when the object is cached", func() {
			var cachedBlobPath string

			BeforeEach(func() {
				cachedBlobPath = filepath.Join(cacheDir, "hello")
				Expect(ioutil.WriteFile(cachedBlobPath, []byte("cached content"), 0666)).To(Succeed())
			})

			It("returns a reader to the cached object", func() {
				reader, _, err := streamer.Stream(logger, "hello")
				Expect(err).ToNot(HaveOccurred())

				readerContent := make([]byte, 14)
				reader.Read(readerContent)
				Expect(readerContent).To(Equal([]byte("cached content")))
			})

			It("doesn't use the internal streamer to stream the original object", func() {
				_, _, err := streamer.Stream(logger, "hello")
				Expect(err).ToNot(HaveOccurred())
				Expect(internalStreamer.StreamCallCount()).To(BeZero())
			})
		})
	})
})

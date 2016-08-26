package fetcher_test

import (
	"errors"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/grootfs/fetcher"
	"code.cloudfoundry.org/grootfs/fetcher/fetcherfakes"
	"code.cloudfoundry.org/grootfs/image_puller/image_pullerfakes"
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
		internalStreamer *image_pullerfakes.FakeStreamer
		cacheDriver      *fetcherfakes.FakeCacheDriver
	)

	BeforeEach(func() {
		var err error
		cacheDriver = new(fetcherfakes.FakeCacheDriver)
		logger = lagertest.NewTestLogger("Streamer")
		cacheDir, err = ioutil.TempDir("", "streamer-cache")
		Expect(err).NotTo(HaveOccurred())

		internalStreamer = new(image_pullerfakes.FakeStreamer)
		streamer = fetcher.NewCachedStreamer(cacheDriver, internalStreamer)
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

		It("returns returns the same reader that the cache driver returns", func() {
			buffer := gbytes.NewBuffer()
			buffer.Write([]byte("hello world"))
			cacheDriver.BlobReturns(buffer, nil)

			reader, _, err := streamer.Stream(logger, "hello")
			Expect(err).ToNot(HaveOccurred())

			readerContent := make([]byte, 11)
			reader.Read(readerContent)
			Expect(readerContent).To(Equal([]byte("hello world")))
		})

		It("calls the cache driver with the correct digest", func() {
			_, _, err := streamer.Stream(logger, "hello")
			Expect(err).ToNot(HaveOccurred())

			Expect(cacheDriver.BlobCallCount()).To(Equal(1))

			_, receivedId, _ := cacheDriver.BlobArgsForCall(0)
			Expect(receivedId).To(Equal("hello"))
		})

		It("forwards the correct streamer to the cache driver", func() {
			_, _, err := streamer.Stream(logger, "hello")
			Expect(err).ToNot(HaveOccurred())

			Expect(cacheDriver.BlobCallCount()).To(Equal(1))

			_, _, streamBlobFunc := cacheDriver.BlobArgsForCall(0)

			contentReader, err := streamBlobFunc(logger)
			Expect(err).ToNot(HaveOccurred())

			Eventually(contentReader).Should(gbytes.Say("hello world"))
		})

		Context("the cache driver fails", func() {
			It("returns an error", func() {
				cacheDriver.BlobReturns(nil, errors.New("super failure"))
				_, _, err := streamer.Stream(logger, "hello")
				Expect(err).To(MatchError(ContainSubstring("super failure")))
			})
		})
	})
})

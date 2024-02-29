package layer_fetcher_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"io"
	"strings"

	"code.cloudfoundry.org/grootfs/fetcher/layer_fetcher"
)

var _ = Describe("QuotaedReader", func() {
	var (
		delegate io.Reader
		quota    int64
		qr       *layer_fetcher.QuotaedReader
	)

	BeforeEach(func() {
		quota = 20
	})

	JustBeforeEach(func() {
		qr = &layer_fetcher.QuotaedReader{
			DelegateReader: io.NopCloser(delegate),
			QuotaLeft:      quota,
			QuotaExceededErrorHandler: func() error {
				return fmt.Errorf("err-quota-exceeded")
			},
		}
	})

	Describe("Read", func() {
		Context("when the underlying reader has less bytes than the quota", func() {
			BeforeEach(func() {
				delegate = strings.NewReader("hello")
			})

			It("reads all the data", func() {
				Expect(io.ReadAll(qr)).To(Equal([]byte("hello")))
			})
		})

		Context("when the underlying reader has just as many bytes as the quota", func() {
			BeforeEach(func() {
				delegate = strings.NewReader("12345678901234567890")
			})

			It("reads all the data", func() {
				Expect(io.ReadAll(qr)).To(Equal([]byte("12345678901234567890")))
			})

			It("bytes remaining to quota are zero", func() {
				io.ReadAll(qr)
				Expect(qr.QuotaLeft).To(BeZero())
			})
		})

		Context("when the underlying reader has more bytes than the quota", func() {
			BeforeEach(func() {
				delegate = strings.NewReader("blah blah blah blah blah blah blah blah")
			})

			It("returns an error", func() {
				_, err := io.ReadAll(qr)
				Expect(err).To(MatchError("err-quota-exceeded"))
			})

			It("reads only as many bytes as allowed by the quota plus one", func() {
				b, _ := io.ReadAll(qr)
				Expect(b).To(HaveLen(int(quota + 1)))
			})
		})

		Context("when we pass a negative quota", func() {
			BeforeEach(func() {
				delegate = strings.NewReader("OMG, negative quota!")
				quota = -1
			})

			It("returns an error", func() {
				_, err := io.ReadAll(qr)
				Expect(err).To(MatchError("err-quota-exceeded"))
			})
		})

		Context("when we pass zero quota", func() {
			BeforeEach(func() {
				delegate = strings.NewReader("A")
				quota = 0
			})

			It("returns an error", func() {
				_, err := io.ReadAll(qr)
				Expect(err).To(MatchError("err-quota-exceeded"))
			})
		})
	})
})

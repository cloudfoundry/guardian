package layer_fetcher_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"io"
	"io/ioutil"
	"strings"

	"code.cloudfoundry.org/grootfs/fetcher/layer_fetcher"
)

var _ = Describe("QuotaedReader", func() {
	var (
		delegate       io.Reader
		quota          int64
		qr             *layer_fetcher.QuotaedReader
		skipValidation bool
	)

	BeforeEach(func() {
		quota = 20
		skipValidation = false
	})

	JustBeforeEach(func() {
		qr = &layer_fetcher.QuotaedReader{
			DelegateReader: ioutil.NopCloser(delegate),
			QuotaLeft:      quota,
			SkipValidation: skipValidation,
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
				Expect(ioutil.ReadAll(qr)).To(Equal([]byte("hello")))
			})
		})

		Context("when the underlying reader has just as many bytes as the quota", func() {
			BeforeEach(func() {
				delegate = strings.NewReader("12345678901234567890")
			})

			It("reads all the data", func() {
				Expect(ioutil.ReadAll(qr)).To(Equal([]byte("12345678901234567890")))
			})

			It("bytes remaining to quota are zero", func() {
				ioutil.ReadAll(qr)
				Expect(qr.QuotaLeft).To(BeZero())
			})
		})

		Context("when the underlying reader has more bytes than the quota", func() {
			BeforeEach(func() {
				delegate = strings.NewReader("blah blah blah blah blah blah blah blah")
			})

			It("returns an error", func() {
				_, err := ioutil.ReadAll(qr)
				Expect(err).To(MatchError("err-quota-exceeded"))
			})

			It("reads only as many bytes as allowed by the quota plus one", func() {
				b, _ := ioutil.ReadAll(qr)
				Expect(b).To(HaveLen(int(quota + 1)))
			})
		})

		Context("when we pass a negative quota", func() {
			BeforeEach(func() {
				delegate = strings.NewReader("does not limit")
				quota = -1
			})

			It("reads all the data", func() {
				Expect(ioutil.ReadAll(qr)).To(Equal([]byte("does not limit")))
			})
		})

		Context("when skipValidation is true", func() {
			BeforeEach(func() {
				delegate = strings.NewReader("does not limit")
				skipValidation = true
				quota = 1
			})

			It("reads all the data", func() {
				Expect(ioutil.ReadAll(qr)).To(Equal([]byte("does not limit")))
			})
		})
	})

	Describe("AnyQuotaLeft", func() {
		Context("when there is quota left", func() {
			BeforeEach(func() {
				quota = 10
			})

			It("returns true", func() {
				Expect(qr.AnyQuotaLeft()).To(BeTrue())
			})

			Context("when skipValidation is true", func() {
				BeforeEach(func() {
					skipValidation = true
				})

				It("returns false", func() {
					Expect(qr.AnyQuotaLeft()).To(BeFalse())
				})
			})
		})
		Context("when there is no quota left", func() {
			BeforeEach(func() {
				quota = 0
			})

			It("returns false", func() {
				Expect(qr.AnyQuotaLeft()).To(BeFalse())
			})

			Context("when skipValidation is true", func() {
				BeforeEach(func() {
					skipValidation = true
				})

				It("returns false", func() {
					Expect(qr.AnyQuotaLeft()).To(BeFalse())
				})
			})
		})
	})
})

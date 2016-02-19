package dns_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/cloudfoundry-incubator/guardian/kawasaki/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("RootfsWriter", func() {
	var (
		rootfsPath string

		rootfsWriter *dns.RootfsWriter
		rootUid      int
		rootGid      int

		log lager.Logger
	)

	BeforeEach(func() {
		rootUid = 40000
		rootGid = 40000

		dns.ChownFunc = func(_ string, _, _ int) error {
			return nil
		}
	})

	JustBeforeEach(func() {
		rootfsWriter = &dns.RootfsWriter{
			RootfsPath: rootfsPath,
			RootUid:    rootUid,
			RootGid:    rootGid,
		}

		log = lagertest.NewTestLogger("test")
	})

	Describe("WriteFile", func() {
		Context("when the root path exists", func() {
			BeforeEach(func() {
				var err error

				rootfsPath, err = ioutil.TempDir("", "")
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				Expect(os.RemoveAll(rootfsPath)).To(Succeed())
			})

			It("should write the file there", func() {
				Expect(rootfsWriter.WriteFile(log, "/test/file.txt", []byte("Hello world"))).To(Succeed())

				filePath := filepath.Join(rootfsPath, "test/file.txt")
				Expect(filePath).To(BeARegularFile())

				f, err := os.Open(filePath)
				Expect(err).NotTo(HaveOccurred())
				contents, err := ioutil.ReadAll(f)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("Hello world"))
			})

			It("should apply the correct ownership", func() {
				filePath := filepath.Join(rootfsPath, "/test/file.txt")

				calledCount := 0
				dns.ChownFunc = func(path string, uid, gid int) error {
					calledCount++
					Expect(path).To(Equal(filePath))
					Expect(uid).To(Equal(rootUid))
					Expect(gid).To(Equal(rootGid))

					return nil
				}

				Expect(rootfsWriter.WriteFile(log, "/test/file.txt", []byte("Hello world"))).To(Succeed())

				Expect(calledCount).To(Equal(1))
			})

			Context("when chowing fails", func() {
				BeforeEach(func() {
					dns.ChownFunc = func(_ string, _, _ int) error {
						return errors.New("banana chown")
					}
				})

				It("should return the error", func() {
					Expect(
						rootfsWriter.WriteFile(log, "file.txt", []byte("Hello world")),
					).To(MatchError(ContainSubstring("banana chown")))
				})
			})
		})

		Context("when the root path does not exist", func() {
			BeforeEach(func() {
				rootfsPath = "/does/not/exist"
			})

			It("should return an error", func() {
				Expect(
					rootfsWriter.WriteFile(log, "file.txt", []byte("Hello world")),
				).To(MatchError(ContainSubstring("/does/not/exist")))
			})
		})
	})
})

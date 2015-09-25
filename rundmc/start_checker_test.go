package rundmc_test

import (
	"io"
	"time"

	"github.com/cloudfoundry-incubator/guardian/rundmc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StartChecker", func() {
	var (
		checker    *rundmc.StartChecker
		pipeReader io.Reader
		pipeWriter io.Writer
	)

	BeforeEach(func() {
		checker = &rundmc.StartChecker{
			Expect: "potato", Timeout: 100 * time.Millisecond,
		}

		pipeReader, pipeWriter = io.Pipe()
	})

	Context("when the expected string is output before the timeout", func() {
		It("returns nil", func() {
			go pipeWriter.Write([]byte("potato"))
			Expect(checker.Check(pipeReader)).To(Succeed())
		})
	})

	Context("when an unexpected string is output before the timeout", func() {
		It("returns an error", func() {
			go pipeWriter.Write([]byte("jamjamjamjam"))
			Expect(checker.Check(pipeReader)).NotTo(Succeed())
		})
	})

	Context("when no output is produced before the timeout", func() {
		It("returns an error", func() {
			Expect(checker.Check(pipeReader)).NotTo(Succeed())
		})
	})
})

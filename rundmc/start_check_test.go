package rundmc_test

import (
	"io"
	"time"

	"github.com/cloudfoundry-incubator/guardian/rundmc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("StdoutCheck", func() {
	Context("when the Expect string is printed to stdout before the timeout", func() {
		It("returns nil", func() {
			check := &rundmc.StdoutCheck{
				"potato", 100 * time.Millisecond,
			}

			sR, sW := io.Pipe()
			go sW.Write([]byte("potato"))
			Expect(check.Check(sR, gbytes.NewBuffer())).To(Succeed())
		})
	})

	Context("when a string is printed to stdout, but it doesnt match", func() {
		It("returns nil", func() {
			check := &rundmc.StdoutCheck{
				"potato", 100 * time.Millisecond,
			}

			sR, sW := io.Pipe()
			go func() {
				sW.Write([]byte("jamjamjamjam"))
			}()
			Expect(check.Check(sR, gbytes.NewBuffer())).NotTo(Succeed())
		})
	})

	Context("when the Expect string is not printed to stdout before the timeout", func() {
		Context("and text has been printed to stderr", func() {
			It("returns an error containing the stderr text", func() {
				check := &rundmc.StdoutCheck{
					"potato", 100 * time.Millisecond,
				}

				sR, sW := io.Pipe()
				go func() {
					time.Sleep(1 * time.Second)
					sW.Write([]byte("potato"))
				}()

				Expect(check.Check(sR, gbytes.BufferWithBytes([]byte("blammo")))).To(MatchError("blammo"))
			})
		})
	})
})

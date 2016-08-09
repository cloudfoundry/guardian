package streamer_test

import (
	"io"
	"os"

	streamerpkg "code.cloudfoundry.org/grootfs/cloner/streamer"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CommandReader", func() {
	var (
		pipeR         io.ReadCloser
		pipeW         io.WriteCloser
		logger        lager.Logger
		commandReader *streamerpkg.CommandReader
		waitFunction  func() error
	)

	BeforeEach(func() {
		var err error

		pipeR, pipeW, err = os.Pipe()
		Expect(err).ToNot(HaveOccurred())

		logger = lagertest.NewTestLogger("command-streamer")

		waitFunction = func() error {
			return nil
		}
	})

	JustBeforeEach(func() {
		commandReader = streamerpkg.NewCommandReader(logger, waitFunction, pipeR)
	})

	Describe("Read", func() {
		BeforeEach(func() {
			pipeW.Write([]byte("hello"))
			Expect(pipeW.Close()).To(Succeed())
		})

		It("streams using the internal streamer", func() {
			buffer := make([]byte, 1024)
			_, err := commandReader.Read(buffer)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(buffer[:5])).To(Equal("hello"))
		})

		It("returns the size of the internal streamer stream", func() {
			buffer := make([]byte, 1024)
			n, err := commandReader.Read(buffer)
			Expect(err).ToNot(HaveOccurred())
			Expect(n).To(Equal(5))
		})

		Context("when the internal streamer gets closed", func() {
			It("returns an error", func() {
				Expect(pipeR.Close()).To(Succeed())

				_, err := commandReader.Read([]byte{})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Close", func() {
		It("closes the streamer", func(done Done) {
			Expect(commandReader.Close()).To(Succeed())

			buffer := make([]byte, 1024)
			_, err := commandReader.Read(buffer)
			Expect(err).To(HaveOccurred())

			close(done)
		}, 1.0)

		Context("when the wait function takes time", func() {
			var waitChan chan struct{}

			BeforeEach(func() {
				waitChan = make(chan struct{})
				waitFunction = func() error {
					<-waitChan
					return nil
				}
			})

			It("blocks until the wait function finishes", func() {
				done := make(chan struct{})
				go func() {
					defer GinkgoRecover()

					Expect(commandReader.Close()).To(Succeed())
					close(done)
				}()

				Consistently(done).ShouldNot(BeClosed())

				close(waitChan)
				Eventually(done).Should(BeClosed())
			})
		})
	})
})

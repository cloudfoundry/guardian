package writer_test

import (
	"strings"

	"github.com/cloudfoundry-incubator/guardian/rundmc/process_tracker/writer"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FanIn", func() {

	var fanIn writer.FanIn
	var fWriter *fakeWriter
	var testBytes []byte

	BeforeEach(func() {
		fanIn = writer.NewFanIn()
		fWriter = &fakeWriter{
			nWriteReturn: 10,
		}
		testBytes = []byte{12}
	})

	It("writes data to a sink and leaves the sink open", func() {
		fanIn.AddSink(fWriter)
		n, err := fanIn.Write(testBytes)
		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(10))
		Expect(fWriter.writeCalls()).To(Equal(1))
		Expect(fWriter.writeArgument()).To(Equal(testBytes))
		Expect(fWriter.closeCalls()).To(Equal(0))

		By("and more data can be written to the sink")
		testBytes2 := []byte{1, 2}
		n, err = fanIn.Write(testBytes2)
		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(10))
		Expect(fWriter.writeArgument()).To(Equal(testBytes2))
		Expect(fWriter.closeCalls()).To(Equal(0))
	})

	It("blocks writes until a sink is added", func() {
		nChan := make(chan int)
		errChan := make(chan error)

		go func() {
			n, err := fanIn.Write(testBytes)
			nChan <- n
			errChan <- err
		}()

		fanIn.AddSink(fWriter)
		n := <-nChan
		err := <-errChan

		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(10))
	})

	It("reads data from a source and writes to a sink", func() {
		fanIn.AddSink(fWriter)
		fanIn.AddSource(strings.NewReader("abcdefghij"))

		Eventually(fWriter.writeCalls).Should(Equal(1))
		Expect(fWriter.writeArgument()).To(Equal([]byte("abcdefghij")))
	})

	It("closes the sink after writing from a source", func() {
		fanIn.AddSink(fWriter)
		fanIn.AddSource(strings.NewReader("abcdefghij"))
		Eventually(fWriter.closeCalls).Should(Equal(1))
	})

	It("it doesn't close a sink when writing from source fails", func() {
		fanIn.AddSink(fWriter)
		fanIn.AddSource(strings.NewReader("a string longer than writer return length"))
		Eventually(fWriter.closeCalls).Should(Equal(0))
	})

	It("closes a sink", func() {
		fanIn.AddSink(fWriter)
		err := fanIn.Close()
		Expect(err).ToNot(HaveOccurred())
		Expect(fWriter.closeCalls()).To(Equal(1))
	})

	It("blocks close until a sink is added", func() {
		errChan := make(chan error)

		go func() {
			err := fanIn.Close()
			errChan <- err
		}()

		fanIn.AddSink(fWriter)
		err := <-errChan

		Expect(err).ToNot(HaveOccurred())
		Expect(fWriter.closeCalls()).To(Equal(1))
	})

	It("returns an error if close is called twice", func() {
		fanIn.AddSink(fWriter)

		err := fanIn.Close()
		Expect(err).ToNot(HaveOccurred())

		err = fanIn.Close()
		Expect(err).To(MatchError("already closed"))

		Expect(fWriter.closeCalls()).To(Equal(1))
	})

	It("returns an error if write is called after close", func() {
		fanIn.AddSink(fWriter)

		err := fanIn.Close()
		Expect(err).ToNot(HaveOccurred())

		_, err = fanIn.Write(testBytes)
		Expect(err).To(MatchError("write after close"))

		Expect(fWriter.writeCalls()).To(Equal(0))
	})
})

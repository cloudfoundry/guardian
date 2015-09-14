package writer_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/guardian/rundmc/process_tracker/writer"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FanOut", func() {
	var fanOut writer.FanOut
	var fWriter *fakeWriter
	var testBytes []byte

	BeforeEach(func() {
		fanOut = writer.NewFanOut()
		fWriter = &fakeWriter{
			nWriteReturn: 10,
		}
		testBytes = []byte{12}
	})

	It("writes data to a sink", func() {
		fanOut.AddSink(fWriter)
		n, err := fanOut.Write(testBytes)

		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(1))

		Expect(fWriter.writeArgument()).To(Equal(testBytes))
		Expect(fWriter.writeCalls()).To(Equal(1))
	})

	It("ignores errors when writing to the sink", func() {
		fWriter.errWriteReturn = errors.New("write error")
		fanOut.AddSink(fWriter)
		n, err := fanOut.Write(testBytes)

		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(1))
	})

	It("writes data to two sinks", func() {
		fWriter2 := &fakeWriter{
			nWriteReturn: 10,
		}
		fanOut.AddSink(fWriter2)
		fanOut.AddSink(fWriter)
		n, err := fanOut.Write(testBytes)

		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(1))

		Expect(fWriter.writeArgument()).To(Equal(testBytes))
		Expect(fWriter.writeCalls()).To(Equal(1))

		Expect(fWriter2.writeArgument()).To(Equal(testBytes))
		Expect(fWriter2.writeCalls()).To(Equal(1))
	})

	It("copes when there are no sinks", func() {
		n, err := fanOut.Write(testBytes)

		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(1))
	})
})

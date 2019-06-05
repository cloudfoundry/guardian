package imageplugin_test

import (
	"time"

	"code.cloudfoundry.org/guardian/imageplugin"
	"code.cloudfoundry.org/guardian/imageplugin/imagepluginfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ImagePlugin", func() {
	var fakeLogger *imagepluginfakes.FakeLogger
	var relogger imageplugin.Relogger

	BeforeEach(func() {
		fakeLogger = new(imagepluginfakes.FakeLogger)
		relogger = imageplugin.NewRelogger(fakeLogger)
	})

	It("returns the number of written bytes correctly", func() {
		numOfBytes, err := relogger.Write([]byte(`{"timestamp":"1540895553.922393799","source":"test","message":"relogger.test","log_level":0,"data":{"test_data":"test_value"}}`))

		Expect(err).NotTo(HaveOccurred())
		Expect(numOfBytes).To(Equal(126))
	})

	It("relogs lager debug log entries to the underlying lager logger", func() {
		_, err := relogger.Write([]byte(`{"timestamp":"1540895553.922393799","source":"test","message":"relogger.test","log_level":0,"data":{"test_data":"test_value"}}`))
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeLogger.DebugCallCount()).To(Equal(1))

		actualMsg, data := fakeLogger.DebugArgsForCall(0)
		Expect(actualMsg).To(Equal("relogger.test"))
		Expect(data).To(HaveLen(1))
		Expect(data[0]["test_data"]).To(Equal("test_value"))
		Expect(data[0]["original_timestamp"]).To(BeTemporally("~", time.Unix(1540895553, 922393799), 100*time.Nanosecond))
	})

	It("relogs lager info log entries to the underlying lager logger", func() {
		_, err := relogger.Write([]byte(`{"timestamp":"1540895553.922393799","source":"test","message":"relogger.test","log_level":1,"data":{"test_data":"test_value"}}`))
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeLogger.InfoCallCount()).To(Equal(1))

		actualMsg, data := fakeLogger.InfoArgsForCall(0)
		Expect(actualMsg).To(Equal("relogger.test"))
		Expect(data).To(HaveLen(1))
		Expect(data[0]["test_data"]).To(Equal("test_value"))
		Expect(data[0]["original_timestamp"]).To(BeTemporally("~", time.Unix(1540895553, 922393799), 100*time.Nanosecond))
	})

	It("relogs lager error log entries to the underlying lager logger", func() {
		_, err := relogger.Write([]byte(`{"timestamp":"1540895553.922393799","source":"test","message":"relogger.test","log_level":2,"data":{"test_data":"test_value"}}`))
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeLogger.ErrorCallCount()).To(Equal(1))

		actualMsg, _, data := fakeLogger.ErrorArgsForCall(0)
		Expect(actualMsg).To(Equal("relogger.test"))
		Expect(data).To(HaveLen(1))
		Expect(data[0]["test_data"]).To(Equal("test_value"))
		Expect(data[0]["original_timestamp"]).To(BeTemporally("~", time.Unix(1540895553, 922393799), 100*time.Nanosecond))
	})

	It("relogs lager fatal log entries to the underlying lager logger", func() {
		_, err := relogger.Write([]byte(`{"timestamp":"1540895553.922393799","source":"test","message":"relogger.test","log_level":3,"data":{"test_data":"test_value"}}`))
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeLogger.ErrorCallCount()).To(Equal(1))

		actualMsg, err, data := fakeLogger.ErrorArgsForCall(0)
		Expect(actualMsg).To(Equal("relogger.test"))
		Expect(err).To(BeNil())
		Expect(data).To(HaveLen(1))
		Expect(data[0]["test_data"]).To(Equal("test_value"))
		Expect(data[0]["original_timestamp"]).To(BeTemporally("~", time.Unix(1540895553, 922393799), 100*time.Nanosecond))
	})

	It("skips empty logs", func() {
		_, err := relogger.Write([]byte{})
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeLogger.ErrorCallCount()).To(Equal(0))
		Expect(fakeLogger.InfoCallCount()).To(Equal(0))
		Expect(fakeLogger.DebugCallCount()).To(Equal(0))
	})

	Context("when the entry is not a lager log entry", func() {
		It("wraps it with a lager error log entry", func() {
			_, err := relogger.Write([]byte(`some random log line`))
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLogger.ErrorCallCount()).To(Equal(1))

			actualMsg, _, data := fakeLogger.ErrorArgsForCall(0)
			Expect(actualMsg).To(Equal("error"))
			Expect(data).To(HaveLen(1))
			Expect(data[0]["output"]).To(Equal("some random log line"))
			Expect(data[0]["original_timestamp"]).To(BeNil())
		})
	})
})

package logging_test

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"code.cloudfoundry.org/guardian/logging"
	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FowardRuncLogsToLager", func() {
	var (
		runcLogs   []byte
		lagerLines []string
	)

	JustBeforeEach(func() {
		var lagerLogs bytes.Buffer
		logger := lager.NewLogger("logging-test")
		logger.RegisterSink(lager.NewWriterSink(
			io.MultiWriter(&lagerLogs, GinkgoWriter),
			lager.DEBUG,
		))

		logging.ForwardRuncLogsToLager(logger, "a-tag", runcLogs)

		lagerLines = strings.Split(lagerLogs.String(), "\n")
	})

	Context("when the logs are valid json", func() {
		BeforeEach(func() {
			runcLogs = []byte(`{"msg":"message 1"}
{"msg":"message 2"}`)
		})

		It("forwards every line to lager", func() {
			// lager adds one empty line
			Expect(lagerLines).To(HaveLen(3))
			Expect(lagerMessage(lagerLines[0])).To(Equal("message 1"))
			Expect(lagerMessage(lagerLines[1])).To(Equal("message 2"))
		})

		It("prefixes the lines with the supplied tag", func() {
			Expect(lagerLines[0]).To(ContainSubstring("a-tag"))
		})
	})

	Context("when the logs are not valid json", func() {
		BeforeEach(func() {
			runcLogs = []byte(`{"msg":"a valid entry"}
}weirdStuff{`)
		})

		It("forwards lines to lager up until the poorly-formed line", func() {
			Expect(lagerMessage(lagerLines[0])).To(Equal("a valid entry"))
		})

		It("prints the raw line", func() {
			Expect(lagerMessage(lagerLines[1])).To(ContainSubstring("weirdStuff"))
		})
	})

	Context("when an empty line occurs", func() {
		BeforeEach(func() {
			runcLogs = []byte(`{"msg":"a valid entry"}
`)
		})

		It("does not attempt to parse the empty line", func() {
			// lager adds one empty line
			Expect(lagerLines).To(HaveLen(2))
		})
	})
})

var _ = Describe("WrapWithErrorFromLastLogLine", func() {
	var (
		runcLogs   []byte
		wrappedErr error
	)

	JustBeforeEach(func() {
		wrappedErr = logging.WrapWithErrorFromLastLogLine("a tag", myError{}, runcLogs)
	})

	Context("when the logs are valid json", func() {
		BeforeEach(func() {
			runcLogs = []byte(`{"level":"error","msg":"message 1"}
{"level":"error","msg":"message 2"}`)
		})

		It("returns an error containing the last runc error log message", func() {
			Expect(wrappedErr).To(MatchError("a tag: some-err: message 2"))
		})

		It("preserves the cause", func() {
			Expect(wrappedErr.(logging.WrappedError).Underlying).To(Equal(myError{}))
		})

		Context("when the last log line is not an error", func() {
			BeforeEach(func() {
				runcLogs = []byte(`{"level":"error","msg":"message 1"}
{"level":"debug","msg":"message 2"}`)
			})

			It("returns an error containing the last runc log message", func() {
				Expect(wrappedErr).To(MatchError("a tag: some-err: message 1"))
			})
		})

		Context("when the last error log line is a fatal error", func() {
			BeforeEach(func() {
				runcLogs = []byte(`{"level":"error","msg":"message 1"}
{"level":"fatal","msg":"message 2"}`)
			})

			It("returns an error containing the fatal error message", func() {
				Expect(wrappedErr).To(MatchError("a tag: some-err: message 2"))
			})
		})
	})

	Context("when the last log line is not valid json", func() {
		BeforeEach(func() {
			runcLogs = []byte(`{"level":"error","msg":"a valid entry"}
}weirdStuff{`)
		})

		It("ignores it", func() {
			Expect(wrappedErr).To(MatchError("a tag: some-err: a valid entry"))
		})
	})

	Context("when the last log line is empty", func() {
		BeforeEach(func() {
			runcLogs = []byte(`{"level":"error","msg":"a valid entry"}
`)
		})

		It("ignores it", func() {
			Expect(wrappedErr).To(MatchError("a tag: some-err: a valid entry"))
		})
	})

	Context("when there are no valid log lines", func() {
		BeforeEach(func() {
			runcLogs = []byte(`}weirdStuff{
`)
		})

		It("returns an error with an empty message", func() {
			Expect(wrappedErr).To(MatchError("a tag: some-err: "))
		})
	})
})

func lagerMessage(line string) string {
	var lagerLine struct{ Data struct{ Message string } }
	Expect(json.Unmarshal([]byte(line), &lagerLine)).To(Succeed())
	return lagerLine.Data.Message
}

type myError struct{}

func (myError) Error() string {
	return "some-err"
}

package runrunc_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"code.cloudfoundry.org/guardian/rundmc/runrunc"
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
		logger := lager.NewLogger("runrunc-test")
		logger.RegisterSink(lager.NewWriterSink(
			io.MultiWriter(&lagerLogs, GinkgoWriter),
			lager.DEBUG,
		))

		runrunc.ForwardRuncLogsToLager(logger, "a-tag", runcLogs)

		lagerLines = strings.Split(lagerLogs.String(), "\n")
	})

	Context("when the logs are well-formed logfmt", func() {
		BeforeEach(func() {
			runcLogs = []byte(`time="2017-08-04T08:46:06Z" level=warning msg="signal: killed"
time="2017-08-04T08:46:06Z" level=error msg="container_linux.go:267: starting container process caused \"process_linux.go:348: container init caused \\\"process_linux.go:320: setting cgroup config for procHooks process caused \\\\\\\"The minimum allowed cpu-shares is 2\\\\\\\"\\\"\"\n"
`)
		})

		It("forwards every line to lager", func() {
			Expect(lagerLines).To(HaveLen(3))
			Expect(lagerMessage(lagerLines[0])).To(Equal("signal: killed"))
			Expect(lagerMessage(lagerLines[1])).To(ContainSubstring("The minimum allowed cpu-shares is 2"))
		})

		It("prefixes the lines with the supplied tag", func() {
			Expect(lagerLines[0]).To(ContainSubstring("a-tag"))
		})
	})

	Context("when the logs are not well-formed logfmt", func() {
		BeforeEach(func() {
			runcLogs = []byte(`time="2017-08-04T09:17:53Z" level=warning msg="signal: killed"
time="2017-08-04T09:17:53Z" level=error msg="container_linux.go:265: starting container process caused \"process_linux.go:348: container init caused \\"process_linux.go:320: setting cgroup config for procHooks process caused \\\\"The minimum allowed cpu-shares is 2\\\\"\\"\"
"
`)
		})

		It("forwards lines to lager up until the poorly-formed line", func() {
			Expect(lagerMessage(lagerLines[0])).To(Equal("signal: killed"))
		})

		It("includes the whole log file in a parse error message", func() {
			Expect(lagerMessage(lagerLines[2])).To(ContainSubstring("The minimum allowed cpu-shares is 2"))
		})
	})
})

var _ = Describe("WrapWithErrorFromLastLogLine", func() {
	var (
		runcLogs   []byte
		wrappedErr error
	)

	JustBeforeEach(func() {
		wrappedErr = runrunc.WrapWithErrorFromLastLogLine("a tag", errors.New("some-err"), runcLogs)
	})

	Context("when the logs are well-formed logfmt", func() {
		BeforeEach(func() {
			runcLogs = []byte(`time="2017-08-04T08:46:06Z" level=warning msg="signal: killed"
time="2017-08-04T08:46:06Z" level=error msg="some message"
`)
		})

		It("returns an error containing the last runc log message", func() {
			Expect(wrappedErr).To(MatchError("a tag: some-err: some message"))
		})
	})

	Context("when the logs are not well-formed logfmt", func() {
		BeforeEach(func() {
			runcLogs = []byte(`time="2017-08-04T09:17:53Z" level=warning msg="signal: killed"
time="2017-08-04T09:17:53Z" level=error msg="container_linux.go:265: starting container process caused \"process_linux.go:348: container init caused \\"process_linux.go:320: setting cgroup config for procHooks process caused \\\\"The minimum allowed cpu-shares is 2\\\\"\\"\"
"
`)
		})

		It("returns an error containing the whole runc log file", func() {
			Expect(wrappedErr).To(MatchError(ContainSubstring("a tag: some-err: ")))
			Expect(wrappedErr).To(MatchError(ContainSubstring("The minimum allowed cpu-shares is 2")))
		})
	})
})

func lagerMessage(line string) string {
	var lagerLine struct{ Data struct{ Message string } }
	Expect(json.Unmarshal([]byte(line), &lagerLine)).To(Succeed())
	return lagerLine.Data.Message
}

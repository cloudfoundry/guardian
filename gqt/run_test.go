package gqt_test

import (
	"io"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/types"
)

var _ = Describe("Run", func() {
	var client *runner.RunningGarden

	DescribeTable("running a process",
		func(spec garden.ProcessSpec, matchers ...func(actual interface{})) {
			client = startGarden()
			container, err := client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			out := gbytes.NewBuffer()
			proc, err := container.Run(
				spec,
				garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, out),
					Stderr: io.MultiWriter(GinkgoWriter, out),
				})
			Expect(err).NotTo(HaveOccurred())

			exitCode, err := proc.Wait()
			Expect(err).NotTo(HaveOccurred())

			for _, m := range matchers {
				m(&process{exitCode, out})
			}
		},

		Entry("with an absolute path",
			spec("/bin/sh", "-c", "echo hello; exit 12"),
			should(gbytes.Say("hello"), gexec.Exit(12)),
		),

		Entry("with a path to be found in a regular user's path",
			spec("sh", "-c", "echo potato; exit 24"),
			should(gbytes.Say("potato"), gexec.Exit(24)),
		),

		Entry("with a path that doesn't exist",
			spec("potato"),
			shouldNot(gexec.Exit(0)),
		),
	)
})

func should(matchers ...types.GomegaMatcher) func(actual interface{}) {
	return func(actual interface{}) {
		for _, matcher := range matchers {
			Expect(actual).To(matcher)
		}
	}
}

func shouldNot(matchers ...types.GomegaMatcher) func(actual interface{}) {
	return func(actual interface{}) {
		for _, matcher := range matchers {
			Expect(actual).ToNot(matcher)
		}
	}
}

func spec(path string, args ...string) garden.ProcessSpec {
	return garden.ProcessSpec{
		Path: path,
		Args: args,
	}
}

type process struct {
	exitCode int
	buffer   *gbytes.Buffer
}

func (p *process) ExitCode() int {
	return p.exitCode
}

func (p *process) Buffer() *gbytes.Buffer {
	return p.buffer
}

package matchers

import (
	"io"

	"code.cloudfoundry.org/garden"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/types"

	"fmt"
)

func HaveFile(expected interface{}) types.GomegaMatcher {
	s, ok := expected.(string)
	if !ok {
		panic("HaveFile matcher expects a string")
	}

	return &haveFileMatcher{
		expected: s,
	}
}

type haveFileMatcher struct {
	expected string
}

func (matcher *haveFileMatcher) Match(actual interface{}) (success bool, err error) {
	container, ok := actual.(garden.Container)
	if !ok {
		return false, fmt.Errorf("HaveFile matcher expects an garden.Container")
	}

	out := gbytes.NewBuffer()
	proc, err := container.Run(
		garden.ProcessSpec{
			Path: "ls",
			Args: []string{matcher.expected},
		},
		garden.ProcessIO{
			Stdout: io.MultiWriter(ginkgo.GinkgoWriter, out),
			Stderr: io.MultiWriter(ginkgo.GinkgoWriter, out),
		})
	if err != nil {
		return false, err
	}

	exitCode, err := proc.Wait()
	if err != nil {
		return false, err
	}
	if exitCode != 0 {
		return false, nil
	}

	return true, nil
}

func (matcher *haveFileMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Container:\n\t%s\nDoes not have file:\n\t%s", matcher.containerHandle(actual), matcher.expected)
}

func (matcher *haveFileMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Container:\n\t%s\nHas file:\n\t%s\nWhen it shouldn't", matcher.containerHandle(actual), matcher.expected)
}

func (matcher *haveFileMatcher) containerHandle(actual interface{}) string {
	container, ok := actual.(garden.Container)
	if !ok {
		panic("HaveFile matcher expects a string")
	}
	return container.Handle()
}

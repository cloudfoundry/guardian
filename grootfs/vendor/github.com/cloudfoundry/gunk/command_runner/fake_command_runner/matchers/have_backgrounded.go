package fake_command_runner_matchers

import (
	"fmt"
	"os/exec"

	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
)

func HaveBackgrounded(spec fake_command_runner.CommandSpec) *HaveBackgroundedMatcher {
	return &HaveBackgroundedMatcher{Spec: spec}
}

type HaveBackgroundedMatcher struct {
	Spec         fake_command_runner.CommandSpec
	backgrounded []*exec.Cmd
}

func (m *HaveBackgroundedMatcher) Match(actual interface{}) (bool, error) {
	runner, ok := actual.(*fake_command_runner.FakeCommandRunner)
	if !ok {
		return false, fmt.Errorf("Not a fake command runner: %#v.", actual)
	}

	m.backgrounded = runner.BackgroundedCommands()

	matched := false
	for _, cmd := range m.backgrounded {
		if m.Spec.Matches(cmd) {
			matched = true
			break
		}
	}

	if matched {
		return true, nil
	} else {
		return false, nil
	}
}

func (m *HaveBackgroundedMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected to background:%s\n\nActually backgrounded:%s", prettySpec(m.Spec), prettyCommands(m.backgrounded))
}

func (m *HaveBackgroundedMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected to not background the following commands:%s", prettySpec(m.Spec))
}

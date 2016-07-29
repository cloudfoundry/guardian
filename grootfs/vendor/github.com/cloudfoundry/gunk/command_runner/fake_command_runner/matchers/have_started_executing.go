package fake_command_runner_matchers

import (
	"fmt"
	"os/exec"

	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
)

func HaveStartedExecuting(spec fake_command_runner.CommandSpec) *HaveStartedExecutingMatcher {
	return &HaveStartedExecutingMatcher{Spec: spec}
}

type HaveStartedExecutingMatcher struct {
	Spec    fake_command_runner.CommandSpec
	started []*exec.Cmd
}

func (m *HaveStartedExecutingMatcher) Match(actual interface{}) (bool, error) {
	runner, ok := actual.(*fake_command_runner.FakeCommandRunner)
	if !ok {
		return false, fmt.Errorf("Not a fake command runner: %#v.", actual)
	}

	m.started = runner.StartedCommands()

	matched := false
	for _, cmd := range m.started {
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

func (m *HaveStartedExecutingMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected to start:%s\n\nActually started:%s", prettySpec(m.Spec), prettyCommands(m.started))
}

func (m *HaveStartedExecutingMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected to not start the following commands:%s", prettySpec(m.Spec))
}

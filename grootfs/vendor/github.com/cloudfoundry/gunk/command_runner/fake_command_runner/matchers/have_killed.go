package fake_command_runner_matchers

import (
	"fmt"
	"os/exec"

	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
)

func HaveKilled(spec fake_command_runner.CommandSpec) *HaveKilledMatcher {
	return &HaveKilledMatcher{Spec: spec}
}

type HaveKilledMatcher struct {
	Spec   fake_command_runner.CommandSpec
	killed []*exec.Cmd
}

func (m *HaveKilledMatcher) Match(actual interface{}) (bool, error) {
	runner, ok := actual.(*fake_command_runner.FakeCommandRunner)
	if !ok {
		return false, fmt.Errorf("Not a fake command runner: %#v.", actual)
	}

	m.killed = runner.KilledCommands()

	matched := false
	for _, cmd := range m.killed {
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

func (m *HaveKilledMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected to kill:%s\n\nActually killed:%s", prettySpec(m.Spec), prettyCommands(m.killed))
}

func (m *HaveKilledMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected to not kill the following commands:%s", prettySpec(m.Spec))
}

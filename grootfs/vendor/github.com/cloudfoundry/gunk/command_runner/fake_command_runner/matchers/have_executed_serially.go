package fake_command_runner_matchers

import (
	"fmt"
	"os/exec"

	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
)

func HaveExecutedSerially(specs ...fake_command_runner.CommandSpec) *HaveExecutedSeriallyMatcher {
	return &HaveExecutedSeriallyMatcher{Specs: specs}
}

type HaveExecutedSeriallyMatcher struct {
	Specs    []fake_command_runner.CommandSpec
	executed []*exec.Cmd
}

func (m *HaveExecutedSeriallyMatcher) Match(actual interface{}) (bool, error) {
	runner, ok := actual.(*fake_command_runner.FakeCommandRunner)
	if !ok {
		return false, fmt.Errorf("Not a fake command runner: %#v.", actual)
	}

	m.executed = runner.ExecutedCommands()

	matched := false
	startSearch := 0

	for _, spec := range m.Specs {
		matched = false

		for i := startSearch; i < len(m.executed); i++ {
			startSearch++

			if !spec.Matches(m.executed[i]) {
				continue
			}

			matched = true

			break
		}

		if !matched {
			break
		}
	}

	if matched {
		return true, nil
	} else {
		return false, nil
	}
}

func (m *HaveExecutedSeriallyMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected to execute:%s\n\nActually executed:%s", prettySpecs(m.Specs), prettyCommands(m.executed))
}

func (m *HaveExecutedSeriallyMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected to not execute the following commands:%s", prettySpecs(m.Specs))
}

package fake_command_runner_matchers

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
)

func HaveSignalled(spec fake_command_runner.CommandSpec, signal os.Signal) *HaveSignalledMatcher {
	return &HaveSignalledMatcher{Spec: spec, Signal: signal}
}

type HaveSignalledMatcher struct {
	Spec              fake_command_runner.CommandSpec
	Signal            os.Signal
	actuallySignalled []*exec.Cmd
}

func (m *HaveSignalledMatcher) Match(actual interface{}) (bool, error) {
	runner, ok := actual.(*fake_command_runner.FakeCommandRunner)
	if !ok {
		return false, fmt.Errorf("Not a fake command runner: %#v.", actual)
	}

	signalled := runner.SignalledCommands()

	matched := false
	for cmd, signal := range signalled {
		if m.Spec.Matches(cmd) {
			matched = signal == m.Signal
			break
		}
	}

	m.actuallySignalled = []*exec.Cmd{}
	for cmd, _ := range signalled {
		m.actuallySignalled = append(m.actuallySignalled, cmd)
	}

	if matched {
		return true, nil
	} else {
		return false, nil
	}
}

func (m *HaveSignalledMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected to signal %s to:%s\n\nActually signalled:%s", m.Signal, prettySpec(m.Spec), prettyCommands(m.actuallySignalled))
}

func (m *HaveSignalledMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected to not signal %s to the following commands:%s", m.Signal, prettySpec(m.Spec))
}

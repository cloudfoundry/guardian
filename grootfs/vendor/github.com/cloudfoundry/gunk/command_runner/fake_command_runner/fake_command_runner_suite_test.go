package fake_command_runner_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestFakeCommandRunner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FakeCommandRunner Suite")
}

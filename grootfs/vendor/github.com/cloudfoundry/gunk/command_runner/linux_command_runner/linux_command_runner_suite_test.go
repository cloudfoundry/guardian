package linux_command_runner_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLinuxCommandRunner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Linux Command Runner Suite")
}

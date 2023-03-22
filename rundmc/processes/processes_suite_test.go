package processes_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestProcesses(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Processes Suite")
}

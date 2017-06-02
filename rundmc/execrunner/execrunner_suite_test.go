package execrunner_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestExecrunner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Execrunner Suite")
}

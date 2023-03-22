package execrunner_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestExecrunner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Execrunner Suite")
}

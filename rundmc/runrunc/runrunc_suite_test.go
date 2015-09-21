package runrunc_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRunrunc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Runrunc Suite")
}

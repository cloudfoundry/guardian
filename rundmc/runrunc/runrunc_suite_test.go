package runrunc_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRunrunc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Runrunc Suite")
}

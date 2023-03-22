package pid_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPid(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pid Suite")
}

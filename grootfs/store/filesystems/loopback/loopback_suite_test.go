package loopback_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestLoopback(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Loopback Suite")
}

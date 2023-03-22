package ports_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPorts(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ports Suite")
}

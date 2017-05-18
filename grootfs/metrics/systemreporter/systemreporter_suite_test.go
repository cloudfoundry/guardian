package systemreporter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSystemreporter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Systemreporter Suite")
}

package dadoo_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDadoo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "rundmc/execrunner/dadoo Suite")
}

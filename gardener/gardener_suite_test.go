package gardener_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGardener(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gardener Suite")
}

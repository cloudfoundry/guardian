package peas_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPeas(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Peas Suite")
}

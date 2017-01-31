package metrix_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMetrix(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metrix Suite")
}

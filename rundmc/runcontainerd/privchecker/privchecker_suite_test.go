package privchecker_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestPrivchecker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Privchecker Suite")
}

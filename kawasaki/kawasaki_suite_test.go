package kawasaki_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestKawasaki(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Kawasaki Suite")
}

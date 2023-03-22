package privchecker_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPrivchecker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Privreader Suite")
}

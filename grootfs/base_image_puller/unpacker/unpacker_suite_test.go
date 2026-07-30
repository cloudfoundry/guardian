package unpacker_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestUnpacker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Unpacker Suite")
}

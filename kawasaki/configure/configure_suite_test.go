package configure_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConfigure(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Configure Suite")
}

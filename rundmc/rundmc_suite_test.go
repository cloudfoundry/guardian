package rundmc_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRundmc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rundmc Suite")
}

package rundmc_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRundmc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rundmc Suite")
}

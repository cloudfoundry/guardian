package subnets_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSubnets(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Subnets Suite")
}

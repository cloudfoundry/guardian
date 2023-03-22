package cpuentitlement_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGuardiancmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cpu Entitlement Suite")
}

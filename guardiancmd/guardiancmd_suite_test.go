package guardiancmd_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGuardiancmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Guardiancmd Suite")
}

package vars_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestVars(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Vars Suite")
}

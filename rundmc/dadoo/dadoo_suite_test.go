package dadoo_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDadoo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dadoo Suite")
}

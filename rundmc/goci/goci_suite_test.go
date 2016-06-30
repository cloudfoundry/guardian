package goci_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGoci(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Goci Suite")
}

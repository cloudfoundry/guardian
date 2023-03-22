package goci_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGoci(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Goci Suite")
}

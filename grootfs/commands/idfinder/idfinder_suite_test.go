package idfinder_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestIdfinder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Idfinder Suite")
}

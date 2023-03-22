package idfinder_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestIdfinder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Idfinder Suite")
}

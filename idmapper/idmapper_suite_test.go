package idmapper_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestIdmapper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Idmapper Suite")
}

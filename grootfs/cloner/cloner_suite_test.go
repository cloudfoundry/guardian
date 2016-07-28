package cloner_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCloner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cloner Suite")
}

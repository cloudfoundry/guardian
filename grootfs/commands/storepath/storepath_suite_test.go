package storepath_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestStorepath(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Storepath Suite")
}

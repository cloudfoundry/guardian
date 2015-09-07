package gqt_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGqt(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gqt Suite")
}

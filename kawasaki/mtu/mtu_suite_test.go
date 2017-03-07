package mtu_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMtu(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Mtu Suite")
}

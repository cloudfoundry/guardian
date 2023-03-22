package mtu_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMtu(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Mtu Suite")
}

package netns_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestNetns(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Netns Suite")
}

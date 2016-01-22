package netplugin_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestNetplugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Netplugin Suite")
}

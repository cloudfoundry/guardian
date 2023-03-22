package netplugin_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestNetplugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Netplugin Suite")
}

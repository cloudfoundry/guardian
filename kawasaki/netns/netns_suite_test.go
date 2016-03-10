package netns_test

import (
	"os/user"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestNetns(t *testing.T) {
	BeforeEach(func() {
		if u, err := user.Current(); err == nil && u.Uid != "0" {
			Skip("Netns suite requires root to run")
		}
	})

	RegisterFailHandler(Fail)
	RunSpecs(t, "Netns Suite")
}

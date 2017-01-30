package image_cloner_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRoottests(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeEach(func() {
		if os.Getuid() != 0 {
			Skip("This suite is only running as root")
		}
	})

	RunSpecs(t, "ROOT: Image Cloner Suite")
}

package guardiancmd_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGuardianCmd(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeEach(func() {
		skip := os.Getenv("GARDEN_TEST_ROOTFS") == ""
		if skip {
			Skip("guardiancmd requires linux")
		}
	})

	RunSpecs(t, "GuardianCmd Suite")
}

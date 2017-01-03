package remote_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var (
	RegistryUsername string
	RegistryPassword string
)

func TestRemote(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeEach(func() {
		RegistryUsername = os.Getenv("REGISTRY_USERNAME")
		RegistryPassword = os.Getenv("REGISTRY_PASSWORD")
	})

	RunSpecs(t, "Remote Suite")
}

package source_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

var (
	RegistryUsername string
	RegistryPassword string
)

func TestSource(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeEach(func() {
		RegistryUsername = os.Getenv("DOCKER_REGISTRY_USERNAME")
		RegistryPassword = os.Getenv("DOCKER_REGISTRY_PASSWORD")
	})

	RunSpecs(t, "Layer Fetcher Source Suite")
}

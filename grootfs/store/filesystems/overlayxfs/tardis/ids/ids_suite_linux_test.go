package ids_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var (
	StorePath string
)

func TestOverlayxfs(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeEach(func() {
		StorePath = fmt.Sprintf("/mnt/xfs-%d", GinkgoParallelNode())
	})

	RunSpecs(t, "Tardis/ids Suite")
}

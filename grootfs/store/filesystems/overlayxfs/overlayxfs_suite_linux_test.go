package overlayxfs_test

import (
	"fmt"
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var (
	StorePath string
)

func TestOverlayxfs(t *testing.T) {
	RegisterFailHandler(Fail)
	rand.Seed(time.Now().UnixNano())

	BeforeEach(func() {
		StorePath = fmt.Sprintf("/mnt/xfs-%d", GinkgoParallelNode())
	})

	RunSpecs(t, "Overlay+Xfs Driver Suite")
}

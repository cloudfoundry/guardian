package overlayxfs_test

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var (
	StorePath          string
	XFSQuotaCalledFile *os.File
	XFSQuotaBin        *os.File
	XFSProgsPath       string
)

func TestOverlayxfs(t *testing.T) {
	RegisterFailHandler(Fail)
	rand.Seed(time.Now().UnixNano())

	BeforeEach(func() {
		XFSProgsPath, XFSQuotaBin, XFSQuotaCalledFile = integration.CreateFakeBin("xfs_quota")

		StorePath = fmt.Sprintf("/mnt/xfs-%d", GinkgoParallelNode())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(XFSProgsPath)).To(Succeed())
	})

	RunSpecs(t, "Overlay+Xfs Driver Suite")
}

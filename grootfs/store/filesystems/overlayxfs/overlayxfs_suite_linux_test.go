package overlayxfs_test

import (
	"fmt"

	"code.cloudfoundry.org/grootfs/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var (
	StorePath     string
	TardisBinPath string
)

func TestOverlayxfs(t *testing.T) {
	RegisterFailHandler(Fail)

	testhelpers.ReseedRandomNumberGenerator()

	BeforeEach(func() {
		StorePath = fmt.Sprintf("/mnt/xfs-%d", GinkgoParallelNode())
	})

	BeforeSuite(func() {
		var err error
		TardisBinPath, err = gexec.Build("code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs/tardis")
		Expect(err).NotTo(HaveOccurred())
		testhelpers.SuidBinary(TardisBinPath)
	})

	RunSpecs(t, "Overlay+Xfs Driver Suite")
}

package peas_test

import (
	"os"

	"code.cloudfoundry.org/guardian/rundmc/peas"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var _ = SynchronizedAfterSuite(func() {}, func() {
	Expect(os.RemoveAll(peas.RootfsPath)).To(Succeed())
})

func TestPeas(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Peas Suite")
}

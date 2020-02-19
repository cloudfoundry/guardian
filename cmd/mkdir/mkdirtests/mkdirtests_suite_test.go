package mkdirtests_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	mkdirbinpath string
)

func TestMkdirtests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Mkdirtests Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	binpath, err := gexec.Build("code.cloudfoundry.org/guardian/cmd/mkdir", "-mod=vendor")
	Expect(err).NotTo(HaveOccurred())
	return []byte(binpath)
}, func(in []byte) {
	mkdirbinpath = string(in)
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})

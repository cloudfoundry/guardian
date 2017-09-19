package socket2me_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var socket2MeBinPath string

var _ = SynchronizedBeforeSuite(func() []byte {
	binPath, err := gexec.Build("code.cloudfoundry.org/guardian/cmd/socket2me")
	Expect(err).NotTo(HaveOccurred())
	return []byte(binPath)
}, func(binPath []byte) {
	socket2MeBinPath = string(binPath)
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})

func TestSocket2Me(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Socket2Me Suite")
}

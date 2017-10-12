package bundlerules_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("NamespaceSharing", func() {
	var (
		containerDir string
		rule         = bundlerules.NamespaceSharing{}
	)

	BeforeEach(func() {
		var err error
		containerDir, err = ioutil.TempDir("", "bundlerules-unit-tests")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(containerDir)).To(Succeed())
	})

	Context("when there is a pidfile in the container dir", func() {
		BeforeEach(func() {
			Expect(ioutil.WriteFile(filepath.Join(containerDir, "pidfile"), []byte("1234"), 0600)).To(Succeed())
		})

		It("shares namespaces with the original container's init process", func() {
			initialBndl := goci.Bundle()
			transformedBndl, err := rule.Apply(initialBndl, gardener.DesiredContainerSpec{}, containerDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(transformedBndl.Namespaces()).To(ConsistOf(
				specs.LinuxNamespace{Type: "mount"},
				specs.LinuxNamespace{Type: "network", Path: "/proc/1234/ns/net"},
				specs.LinuxNamespace{Type: "user", Path: "/proc/1234/ns/user"},
				specs.LinuxNamespace{Type: "ipc", Path: "/proc/1234/ns/ipc"},
				specs.LinuxNamespace{Type: "pid", Path: "/proc/1234/ns/pid"},
				specs.LinuxNamespace{Type: "uts", Path: "/proc/1234/ns/uts"},
			))
		})
	})

	PContext("when there is no pidfile in the container dir", func() {})
})

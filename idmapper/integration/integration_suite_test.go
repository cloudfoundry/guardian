package integration_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path"

	"code.cloudfoundry.org/idmapper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var (
	NewuidmapBin        string
	NewgidmapBin        string
	MaximusBin          string
	NamespaceWrapperBin string

	RootID    = uint32(0)
	NobodyID  = uint32(65534)
	MaximusID uint32
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)

	SynchronizedBeforeSuite(func() []byte {
		bins := make(map[string]string)

		newuidmapBin, err := gexec.Build("code.cloudfoundry.org/idmapper/cmd/newuidmap")
		Expect(err).NotTo(HaveOccurred())
		bins["newuidmapBin"] = newuidmapBin
		fixPermission(path.Dir(newuidmapBin))
		suid(newuidmapBin)

		newgidmapBin, err := gexec.Build("code.cloudfoundry.org/idmapper/cmd/newgidmap")
		Expect(err).NotTo(HaveOccurred())
		bins["newgidmapBin"] = newgidmapBin
		fixPermission(path.Dir(newgidmapBin))
		suid(newgidmapBin)

		maximusBin, err := gexec.Build("code.cloudfoundry.org/idmapper/cmd/maximus")
		Expect(err).NotTo(HaveOccurred())
		bins["maximusBin"] = maximusBin

		namespaceWrapperBin, err := gexec.Build("code.cloudfoundry.org/idmapper/integration/wrapper")
		Expect(err).NotTo(HaveOccurred())
		bins["namespaceWrapperBin"] = namespaceWrapperBin

		data, err := json.Marshal(bins)
		Expect(err).NotTo(HaveOccurred())

		return data
	}, func(data []byte) {
		bins := make(map[string]string)
		Expect(json.Unmarshal(data, &bins)).To(Succeed())

		NewuidmapBin = bins["newuidmapBin"]
		NewgidmapBin = bins["newgidmapBin"]
		MaximusBin = bins["maximusBin"]
		NamespaceWrapperBin = bins["namespaceWrapperBin"]

		MaximusID = uint32(idmapper.Min(idmapper.MustGetMaxValidUID(), idmapper.MustGetMaxValidGID()))
	})

	RunSpecs(t, "Integration Suite")
}

func fixPermission(dirPath string) {
	fi, err := os.Stat(dirPath)
	Expect(err).NotTo(HaveOccurred())
	if !fi.IsDir() {
		return
	}

	// does other have the execute permission?
	if mode := fi.Mode(); mode&01 == 0 {
		Expect(os.Chmod(dirPath, 0755)).To(Succeed())
	}

	if dirPath == "/" {
		return
	}
	fixPermission(path.Dir(dirPath))
}

func suid(binPath string) {
	sess, err := gexec.Start(
		exec.Command("sudo", "chown", "root:root", binPath),
		GinkgoWriter, GinkgoWriter,
	)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))

	sess, err = gexec.Start(
		exec.Command("sudo", "chmod", "u+s", binPath),
		GinkgoWriter, GinkgoWriter,
	)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))
}

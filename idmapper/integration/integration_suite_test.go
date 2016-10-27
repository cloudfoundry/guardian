package integration_test

import (
	"encoding/json"
	"os"
	"os/user"
	"path"
	"strconv"

	"code.cloudfoundry.org/idmapper/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var (
	NewuidmapBin        string
	NewgidmapBin        string
	NamespaceWrapperBin string
	GrootUID            uint32
	GrootGID            uint32

	RootID   = 0
	NobodyID = 65534
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)

	SynchronizedBeforeSuite(func() []byte {
		bins := make(map[string]string)

		newuidmapBin, err := gexec.Build("code.cloudfoundry.org/idmapper/cmd/newuidmap")
		Expect(err).NotTo(HaveOccurred())
		bins["newuidmapBin"] = newuidmapBin
		fixPermission(path.Dir(newuidmapBin))
		testhelpers.Suid(newuidmapBin)

		newgidmapBin, err := gexec.Build("code.cloudfoundry.org/idmapper/cmd/newgidmap")
		Expect(err).NotTo(HaveOccurred())
		bins["newgidmapBin"] = newgidmapBin
		fixPermission(path.Dir(newgidmapBin))
		testhelpers.Suid(newgidmapBin)

		namespaceWrapperBin, err := gexec.Build("code.cloudfoundry.org/idmapper/integration/wrapper")
		Expect(err).NotTo(HaveOccurred())
		bins["namespaceWrapperBin"] = namespaceWrapperBin

		data, err := json.Marshal(bins)
		Expect(err).NotTo(HaveOccurred())

		return data
	}, func(data []byte) {
		bins := make(map[string]string)
		Expect(json.Unmarshal(data, &bins)).To(Succeed())

		grootUser, err := user.Lookup("groot")
		Expect(err).NotTo(HaveOccurred())

		grootUID, err := strconv.ParseInt(grootUser.Uid, 10, 32)
		Expect(err).NotTo(HaveOccurred())
		GrootUID = uint32(grootUID)

		grootGID, err := strconv.ParseInt(grootUser.Gid, 10, 32)
		Expect(err).NotTo(HaveOccurred())
		GrootGID = uint32(grootGID)

		NewuidmapBin = bins["newuidmapBin"]
		NewgidmapBin = bins["newgidmapBin"]
		NamespaceWrapperBin = bins["namespaceWrapperBin"]
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

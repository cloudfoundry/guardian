package idmapper_test

import (
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestIdmapper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Idmapper Suite")
}

func tempDir(dir, prefix string) string {
	workDir, err := ioutil.TempDir("", "")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return workDir
}

func writeFile(path string, data []byte, perm os.FileMode) {
	ExpectWithOffset(1, ioutil.WriteFile(path, data, perm)).To(Succeed())
}

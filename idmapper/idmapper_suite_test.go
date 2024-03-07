package idmapper_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestIdmapper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Idmapper Suite")
}

func tempDir(dir, prefix string) string {
	workDir, err := os.MkdirTemp("", "")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return workDir
}

func writeFile(path string, data []byte, perm os.FileMode) {
	ExpectWithOffset(1, os.WriteFile(path, data, perm)).To(Succeed())
}

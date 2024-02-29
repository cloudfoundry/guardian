package properties_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestProperties(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Properties Suite")
}

func tempDir(dir, prefix string) string {
	path, err := os.MkdirTemp(dir, prefix)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return path
}

func writeFileString(filename, data string, perm os.FileMode) {
	ExpectWithOffset(1, os.WriteFile(filename, []byte(data), perm)).To(Succeed())
}

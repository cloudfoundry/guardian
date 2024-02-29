package runcontainerd_test

import (
	"encoding/json"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRuncontainerd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Runcontainerd Suite")
}

func tempDir(dir, prefix string) string {
	path, err := os.MkdirTemp(dir, prefix)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return path
}

func marshal(v interface{}) []byte {
	content, err := json.Marshal(v)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return content
}

func writeFile(filename string, data []byte, perm os.FileMode) {
	ExpectWithOffset(1, os.WriteFile(filename, data, perm)).To(Succeed())
}

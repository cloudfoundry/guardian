package runcontainerd_test

import (
	"encoding/json"
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRuncontainerd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Runcontainerd Suite")
}

func tempDir(dir, prefix string) string {
	path, err := ioutil.TempDir(dir, prefix)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return path
}

func marshal(v interface{}) []byte {
	content, err := json.Marshal(v)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return content
}

func writeFile(filename string, data []byte, perm os.FileMode) {
	ExpectWithOffset(1, ioutil.WriteFile(filename, data, perm)).To(Succeed())
}

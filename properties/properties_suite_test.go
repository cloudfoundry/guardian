package properties_test

import (
	"io/ioutil"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestProperties(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Properties Suite")
}

func tempDir(dir, prefix string) string {
	path, err := ioutil.TempDir(dir, prefix)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return path
}

func writeFileString(filename, data string, perm os.FileMode) {
	ExpectWithOffset(1, ioutil.WriteFile(filename, []byte(data), perm)).To(Succeed())
}

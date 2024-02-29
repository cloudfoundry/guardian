package cgroups_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCgroups(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cgroups Suite")
}

func readFile(path string) []byte {
	content, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	return content
}

func stat(path string) os.FileInfo {
	info, err := os.Stat(path)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return info
}

func tempDir(dir, prefix string) string {
	name, err := os.MkdirTemp(dir, prefix)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return name
}

func int64ptr(i int64) *int64 {
	return &i
}

type mountArgs struct {
	source string
	target string
	fstype string
	flags  uintptr
	opts   string
}

func newMountArgs(source, target, fstype string, flags uintptr, opts string) mountArgs {
	return mountArgs{source, target, fstype, flags, opts}
}

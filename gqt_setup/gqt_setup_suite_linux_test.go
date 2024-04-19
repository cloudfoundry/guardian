//go:build linux

package gqt_setup_test

import (
	"fmt"
	"os"

	. "github.com/onsi/gomega"
)

// E.g. nodeToString(1) = a, nodeToString(2) = b, etc ...
func nodeToString(ginkgoNode int) string {
	r := 'a' + ginkgoNode - 1
	Expect(r).To(BeNumerically(">=", 'a'))
	Expect(r).To(BeNumerically("<=", 'z'))
	return fmt.Sprintf("%c", r)
}

func readFile(path string) string {
	content, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	return string(content)
}

func cpuThrottlingEnabled() bool {
	return os.Getenv("CPU_THROTTLING_ENABLED") == "true"
}

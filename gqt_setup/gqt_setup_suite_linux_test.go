//go:build linux

package gqt_setup_test

import (
	"fmt"
	"os"
	"strconv"

	. "github.com/onsi/gomega"
)

var (
	// the unprivileged user is baked into the cloudfoundry/garden-runc-release image
	unprivilegedUID = uint32(5000)
	unprivilegedGID = uint32(5000)
)

// E.g. nodeToString(1) = a, nodeToString(2) = b, etc ...
func nodeToString(ginkgoNode int) string {
	r := 'a' + ginkgoNode - 1
	Expect(r).To(BeNumerically(">=", 'a'))
	Expect(r).To(BeNumerically("<=", 'z'))
	return fmt.Sprintf("%c", r)
}

func idToStr(id uint32) string {
	return strconv.FormatUint(uint64(id), 10)
}

func readFile(path string) string {
	content, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	return string(content)
}

func cpuThrottlingEnabled() bool {
	return os.Getenv("CPU_THROTTLING_ENABLED") == "true"
}

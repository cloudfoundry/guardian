package testhelpers

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// ReseedRandomNumberGenerator reinitialises the global random number generator
// with a new seed value, which incorporates the system time and GinkgoParallelProcess
// id. This should prevent random number-related races between tests which kick
// off at the same time on different Ginkgo nodes.
func ReseedRandomNumberGenerator() {
	rand.Seed(time.Now().UnixNano() + int64(GinkgoParallelProcess()*1000))
}

func NewRandomID() string {
	return fmt.Sprintf("random-id-%d", rand.Int())
}

func EnableRootIDMapRange() {
	Expect(enableRootIDMapRange("/etc/subuid")).To(Succeed())
	Expect(enableRootIDMapRange("/etc/subgid")).To(Succeed())
}

func enableRootIDMapRange(mapFilePath string) error {
	f, err := os.OpenFile(mapFilePath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString("root:0:1\n")
	return err
}

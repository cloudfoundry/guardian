package testhelpers

import (
	"fmt"
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
)

// ReseedRandomNumberGenerator reinitialises the global random number generator
// with a new seed value, which incorporates the system time and GinkgoParallelNode
// id. This should prevent random number-related races between tests which kick
// off at the same time on different Ginkgo nodes.
func ReseedRandomNumberGenerator() {
	rand.Seed(time.Now().UnixNano() + int64(GinkgoParallelNode()*1000))
}

func NewRandomID() string {
	return fmt.Sprintf("random-id-%d", rand.Int())
}

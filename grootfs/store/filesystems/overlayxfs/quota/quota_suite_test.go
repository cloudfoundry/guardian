package quota_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var XfsMountPoint string

func TestQuota(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeEach(func() {
		XfsMountPoint = fmt.Sprintf("/mnt/xfs-%d", GinkgoParallelNode())
	})

	RunSpecs(t, "Quota Suite")
}

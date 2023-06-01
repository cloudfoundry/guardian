package quota_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

var XfsMountPointPool chan string

func TestQuota(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeSuite(func() {
		// 10 mount points created in test, assign one per ginkgo worker
		XfsMountPointPool = make(chan string, 10)

		suiteConfig, _ := GinkgoConfiguration()
		if suiteConfig.ParallelTotal > 10 {
			Fail("Ginkgo was run with >10 parallel nodes, but only enough filesystem resources were provisioned for 10. Update ci/scripts/unit_tests.sh or ci/script/stest/utils.sh accordingly")
		}

		// In ginkgo v2, beforesuite runs differently in parallel, so we'll assign only a specific FS to each
		// worker. The tests make use of the pool to add/remove the filesystems as they're available
		XfsMountPointPool <- fmt.Sprintf("/mnt/xfs-%d", GinkgoParallelProcess())
		// 10 mount points created in test
	})

	RunSpecs(t, "Quota Suite")
}

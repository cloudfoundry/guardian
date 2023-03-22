package quota_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

var XfsMountPoint string

func TestQuota(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeEach(func() {
		XfsMountPoint = fmt.Sprintf("/mnt/xfs-%d", GinkgoParallelProcess())
	})

	RunSpecs(t, "Quota Suite")
}

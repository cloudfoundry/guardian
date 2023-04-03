package quota_test

import (
	"fmt"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

var XfsMountPointPool sync.Pool

func TestQuota(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeSuite(func() {
		XfsMountPointPool = sync.Pool{
			New: func() any { return "" },
		}
		// 5 mount points created in test
		for i := 1; i < 5; i++ {
			XfsMountPointPool.Put(fmt.Sprintf("/mnt/xfs-%d", i))
		}
	})

	RunSpecs(t, "Quota Suite")
}

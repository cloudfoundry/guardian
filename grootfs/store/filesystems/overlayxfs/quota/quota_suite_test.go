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
		XfsMountPointPool = make(chan string, 10)
		// 10 mount points created in test
		for i := 1; i < 10; i++ {
			XfsMountPointPool <- fmt.Sprintf("/mnt/xfs-%d", i)
		}
	})

	RunSpecs(t, "Quota Suite")
}

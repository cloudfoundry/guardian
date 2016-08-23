package cache_driver_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCacheDriver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CacheDriver Suite")
}

package cache_driver_test

import (
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCacheDriver(t *testing.T) {
	RegisterFailHandler(Fail)

	rand.Seed(time.Now().UnixNano())
	RunSpecs(t, "CacheDriver Suite")
}

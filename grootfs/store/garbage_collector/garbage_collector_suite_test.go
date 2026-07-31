package garbage_collector_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGarbageCollector(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GarbageCollector Suite")
}

package preparerootfs_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPreparerootfs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Preparerootfs Suite")
}

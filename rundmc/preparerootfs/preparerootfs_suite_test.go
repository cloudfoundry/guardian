package preparerootfs_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPreparerootfs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Preparerootfs Suite")
}

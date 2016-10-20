package dependency_manager_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDependencyManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DependencyManager Suite")
}

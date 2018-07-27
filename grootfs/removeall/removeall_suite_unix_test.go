package removeall_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRemoveall(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Removeall Suite")
}

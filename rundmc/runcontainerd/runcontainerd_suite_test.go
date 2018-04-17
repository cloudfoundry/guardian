package runcontainerd_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRuncontainerd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Runcontainerd Suite")
}

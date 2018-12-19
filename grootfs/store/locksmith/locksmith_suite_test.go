package locksmith_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestManager(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Locksmith Suite")
}

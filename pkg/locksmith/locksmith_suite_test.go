package locksmith_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLocksmith(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Locksmith Suite")
}

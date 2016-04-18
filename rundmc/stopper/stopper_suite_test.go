package stopper_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestStopper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Stopper Suite")
}

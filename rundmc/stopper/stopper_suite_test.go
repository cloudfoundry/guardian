package stopper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestStopper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Stopper Suite")
}

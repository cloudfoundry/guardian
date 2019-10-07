package mount_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestUnmounter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Unmounter Suite")
}

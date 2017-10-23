package filesystems_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestFilesystems(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Filesystems Suite")
}

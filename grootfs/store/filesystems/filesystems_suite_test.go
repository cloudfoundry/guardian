package filesystems_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestFilesystems(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Filesystems Suite")
}

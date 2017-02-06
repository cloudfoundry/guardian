package btrfs

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestVolumedriver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "BTRFS Driver Suite")
}

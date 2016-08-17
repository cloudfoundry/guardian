package volume_driver_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestVolumedriver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Volume Driver Suite")
}

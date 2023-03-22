package image_manager_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestImageManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ImageManager Suite")
}

package image_cloner_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestImageCloner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ImageCloner Suite")
}

package base_image_puller_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBaseImagePuller(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "BaseImage puller suite")
}

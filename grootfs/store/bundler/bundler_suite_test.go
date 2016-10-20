package bundler_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBundler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bundler Suite")
}

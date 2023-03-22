package bundlerules_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBundlerules(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bundlerules Suite")
}

package properties_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestProperties(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Properties Suite")
}

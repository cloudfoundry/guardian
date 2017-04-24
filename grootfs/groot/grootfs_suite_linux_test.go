package groot_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGroot(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Grootfs Suite")
}

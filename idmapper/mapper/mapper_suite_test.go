package mapper_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMappings(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Mapper Suite")
}

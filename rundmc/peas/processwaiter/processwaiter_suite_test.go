package processwaiter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestProcesswaiter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Processwaiter Suite")
}

package processwaiter_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestProcesswaiter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Processwaiter Suite")
}

package deleter_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDeleter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Deleter Suite")
}

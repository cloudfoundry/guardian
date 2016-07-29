package test_server_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTest_server(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test_server Suite")
}

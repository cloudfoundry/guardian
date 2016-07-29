package group_runner_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGroupRunner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GroupRunner Suite")
}

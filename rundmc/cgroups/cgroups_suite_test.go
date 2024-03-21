package cgroups_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCgroups(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cgroups Suite")
}

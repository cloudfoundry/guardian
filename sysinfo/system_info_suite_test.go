package sysinfo_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSystemInfo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SystemInfo Suite")
}

package configure_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestConfigure(t *testing.T) {
	SetDefaultEventuallyTimeout(2 * time.Second)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Configure Suite")
}

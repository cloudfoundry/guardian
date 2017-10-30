package pidreader_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPidreader(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pidreader Suite")
}

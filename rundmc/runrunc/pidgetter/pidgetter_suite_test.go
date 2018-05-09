package pidgetter_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestPidgetter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pidgetter Suite")
}

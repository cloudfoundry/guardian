package depot_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDepot(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Depot Suite")
}

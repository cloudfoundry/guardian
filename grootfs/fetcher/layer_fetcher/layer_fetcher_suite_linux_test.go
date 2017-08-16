package layer_fetcher_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLayerFetcher(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Layer Fetcher Suite")
}

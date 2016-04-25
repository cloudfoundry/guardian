package gardener_test

import (
	"github.com/cloudfoundry-incubator/guardian/gardener"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NoopRestorer", func() {
	It("returns all the handles", func() {
		handles := []string{"banana", "foo"}

		restorer := &gardener.NoopRestorer{}
		returnedHandles := restorer.Restore(nil, handles)

		Expect(returnedHandles).To(Equal(handles))
	})
})

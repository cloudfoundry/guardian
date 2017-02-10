package gardener_test

import (
	"errors"

	. "code.cloudfoundry.org/guardian/gardener"
	fakes "code.cloudfoundry.org/guardian/gardener/gardenerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BulkStarter", func() {
	var (
		bulkStarter  BulkStarter
		fakeStarter1 *fakes.FakeStarter
		fakeStarter2 *fakes.FakeStarter
	)

	BeforeEach(func() {
		fakeStarter1 = new(fakes.FakeStarter)
		fakeStarter2 = new(fakes.FakeStarter)

		bulkStarter = NewBulkStarter([]Starter{
			fakeStarter1,
			fakeStarter2,
		})

	})

	It("calls Start once on each provided garden.Starter", func() {
		Expect(bulkStarter.StartAll()).To(Succeed())

		Expect(fakeStarter1.StartCallCount()).To(Equal(1))
		Expect(fakeStarter2.StartCallCount()).To(Equal(1))
	})

	Context("when one of the garden.Starters returns an error", func() {
		JustBeforeEach(func() {
			fakeStarter2.StartReturns(errors.New("Boom"))
		})
		It("exits with a meaningful error", func() {
			Expect(bulkStarter.StartAll()).To(MatchError("Boom"))
		})

	})
})

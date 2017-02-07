package guardiancmd_test

import (
	"errors"

	"code.cloudfoundry.org/guardian/gardener"
	. "code.cloudfoundry.org/guardian/guardiancmd"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	gardenerfakes "code.cloudfoundry.org/guardian/gardener/gardenerfakes"
)

var _ = Describe("Setup", func() {
	var (
		bulkStarter  BulkStarter
		fakeStarter1 *gardenerfakes.FakeStarter
		fakeStarter2 *gardenerfakes.FakeStarter
	)

	BeforeEach(func() {
		fakeStarter1 = new(gardenerfakes.FakeStarter)
		fakeStarter2 = new(gardenerfakes.FakeStarter)

		bulkStarter = BulkStarter{
			Starters: []gardener.Starter{
				fakeStarter1,
				fakeStarter2,
			},
		}

	})

	It("calls Start once on each provided garden.Starters", func() {
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

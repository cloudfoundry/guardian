package gardener_test

import (
	"errors"

	"code.cloudfoundry.org/guardian/gardener"
	fakes "code.cloudfoundry.org/guardian/gardener/gardenerfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Restorer", func() {
	var (
		fakeNetworker *fakes.FakeNetworker
		restorer      gardener.Restorer
		logger        lager.Logger
	)

	BeforeEach(func() {
		fakeNetworker = new(fakes.FakeNetworker)
		restorer = gardener.NewRestorer(fakeNetworker)
		logger = lagertest.NewTestLogger("test")
	})

	Describe("Restore", func() {
		It("asks the networker to restore settings for each container", func() {
			Expect(restorer.Restore(logger, []string{"foo", "bar"})).To(BeEmpty())

			Expect(fakeNetworker.RestoreCallCount()).To(Equal(2))
			_, handle := fakeNetworker.RestoreArgsForCall(0)
			Expect(handle).To(Equal("foo"))
			_, handle = fakeNetworker.RestoreArgsForCall(1)
			Expect(handle).To(Equal("bar"))
		})

		It("returns the handles that it can't restore", func() {
			fakeNetworker.RestoreStub = func(_ lager.Logger, handle string) error {
				if handle == "bar" {
					return errors.New("banana")
				}

				return nil
			}

			Expect(restorer.Restore(logger, []string{"foo", "bar"})).To(Equal([]string{"bar"}))
		})
	})
})

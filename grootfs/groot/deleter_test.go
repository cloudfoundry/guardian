package groot_test

import (
	"errors"

	grootpkg "code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/groot/grootfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Deleter", func() {
	var (
		fakeBundler           *grootfakes.FakeBundler
		fakeDependencyManager *grootfakes.FakeDependencyManager
		groot                 *grootpkg.Deleter
		logger                lager.Logger
	)

	BeforeEach(func() {
		fakeBundler = new(grootfakes.FakeBundler)
		fakeDependencyManager = new(grootfakes.FakeDependencyManager)

		groot = grootpkg.IamDeleter(fakeBundler, fakeDependencyManager)
		logger = lagertest.NewTestLogger("groot-deleter")
	})

	Describe("Delete", func() {
		It("destroys a bundle", func() {
			Expect(groot.Delete(logger, "some-id")).To(Succeed())

			_, bundleId := fakeBundler.DestroyArgsForCall(0)
			Expect(bundleId).To(Equal("some-id"))
		})

		It("deregisters bundle dependencies", func() {
			Expect(groot.Delete(logger, "some-id")).To(Succeed())
			Expect(fakeDependencyManager.DeregisterCallCount()).To(Equal(1))
		})

		Context("when destroying a bundle fails", func() {
			BeforeEach(func() {
				fakeBundler.DestroyReturns(errors.New("failed to destroy bundle"))
			})

			It("returns an error", func() {
				Expect(groot.Delete(logger, "some-id")).To(MatchError(ContainSubstring("failed to destroy bundle")))
			})
		})
	})

})

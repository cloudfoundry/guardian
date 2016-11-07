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
		fakeImageCloner           *grootfakes.FakeImageCloner
		fakeDependencyManager *grootfakes.FakeDependencyManager
		groot                 *grootpkg.Deleter
		logger                lager.Logger
	)

	BeforeEach(func() {
		fakeImageCloner = new(grootfakes.FakeImageCloner)
		fakeDependencyManager = new(grootfakes.FakeDependencyManager)

		groot = grootpkg.IamDeleter(fakeImageCloner, fakeDependencyManager)
		logger = lagertest.NewTestLogger("groot-deleter")
	})

	Describe("Delete", func() {
		It("destroys a image", func() {
			Expect(groot.Delete(logger, "some-id")).To(Succeed())

			_, imageId := fakeImageCloner.DestroyArgsForCall(0)
			Expect(imageId).To(Equal("some-id"))
		})

		It("deregisters image dependencies", func() {
			Expect(groot.Delete(logger, "some-id")).To(Succeed())
			Expect(fakeDependencyManager.DeregisterCallCount()).To(Equal(1))
		})

		Context("when destroying a image fails", func() {
			BeforeEach(func() {
				fakeImageCloner.DestroyReturns(errors.New("failed to destroy image"))
			})

			It("returns an error", func() {
				Expect(groot.Delete(logger, "some-id")).To(MatchError(ContainSubstring("failed to destroy image")))
			})
		})
	})

})

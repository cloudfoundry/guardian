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

var _ = Describe("I AM GROOT, the Orchestrator", func() {
	var cloner *grootfakes.FakeCloner
	var graph *grootfakes.FakeGraph
	var groot *grootpkg.Groot
	var logger lager.Logger

	BeforeEach(func() {
		cloner = new(grootfakes.FakeCloner)
		graph = new(grootfakes.FakeGraph)
		logger = lagertest.NewTestLogger("groot")
		groot = grootpkg.IamGroot(graph, cloner)
	})

	Describe("Create", func() {
		It("makes a bundle", func() {
			_, err := groot.Create(logger, grootpkg.CreateSpec{
				ID: "some-id",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(graph.MakeBundleCallCount()).To(Equal(1))
			_, id := graph.MakeBundleArgsForCall(0)
			Expect(id).To(Equal("some-id"))
		})

		It("returns the bundle path", func() {
			graph.MakeBundleReturns("/path/to/bundle", nil)

			bundlePath, err := groot.Create(logger, grootpkg.CreateSpec{})
			Expect(err).NotTo(HaveOccurred())
			Expect(bundlePath).To(Equal("/path/to/bundle"))
		})

		Context("when creating the bundle fails", func() {
			BeforeEach(func() {
				graph.MakeBundleReturns("", errors.New("Failed to make bundle"))
			})

			It("returns the error", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{})
				Expect(err).To(MatchError("making bundle: Failed to make bundle"))
			})
		})

		It("clones the image", func() {
			graph.MakeBundleReturns("/path/to/bundle", nil)

			uidMappings := []grootpkg.IDMappingSpec{grootpkg.IDMappingSpec{HostID: 1, NamespaceID: 2, Size: 10}}
			gidMappings := []grootpkg.IDMappingSpec{grootpkg.IDMappingSpec{HostID: 10, NamespaceID: 20, Size: 100}}

			_, err := groot.Create(logger, grootpkg.CreateSpec{
				ImagePath:   "/path/to/image",
				UIDMappings: uidMappings,
				GIDMappings: gidMappings,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(cloner.CloneCallCount()).To(Equal(1))
			_, cloneSpec := cloner.CloneArgsForCall(0)
			Expect(cloneSpec.FromDir).To(Equal("/path/to/image"))
			Expect(cloneSpec.ToDir).To(Equal("/path/to/bundle/rootfs"))
			Expect(cloneSpec.UIDMappings).To(Equal(uidMappings))
			Expect(cloneSpec.GIDMappings).To(Equal(gidMappings))
		})

		Context("when cloning fails", func() {
			BeforeEach(func() {
				cloner.CloneReturns(errors.New("Failed to clone"))
			})

			It("returns the error", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{})
				Expect(err).To(MatchError("cloning: Failed to clone"))
			})
		})
	})
})

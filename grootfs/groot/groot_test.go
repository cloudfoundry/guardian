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
	var (
		cloner     *grootfakes.FakeCloner
		graph      *grootfakes.FakeGraph
		fakeBundle *grootfakes.FakeBundle
		groot      *grootpkg.Groot
		logger     lager.Logger
	)

	BeforeEach(func() {
		cloner = new(grootfakes.FakeCloner)
		graph = new(grootfakes.FakeGraph)
		fakeBundle = new(grootfakes.FakeBundle)

		graph.MakeBundleReturns(fakeBundle, nil)

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
			fakeBundle.PathReturns("/path/to/bundle")

			bundle, err := groot.Create(logger, grootpkg.CreateSpec{})
			Expect(err).NotTo(HaveOccurred())
			Expect(bundle.Path()).To(Equal("/path/to/bundle"))
		})

		Context("when creating the bundle fails", func() {
			BeforeEach(func() {
				graph.MakeBundleReturns(nil, errors.New("Failed to make bundle"))
			})

			It("returns the error", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{})
				Expect(err).To(MatchError("making bundle: Failed to make bundle"))
			})
		})

		It("clones the image", func() {
			fakeBundle.PathReturns("/path/to/bundle")
			fakeBundle.RootFSPathReturns("/path/to/bundle/rootfs")

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

			It("deletes the bundle", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{
					ID: "some-id",
				})
				Expect(err).To(HaveOccurred())

				Expect(graph.DeleteBundleCallCount()).To(Equal(1))
				_, id := graph.DeleteBundleArgsForCall(0)
				Expect(id).To(Equal("some-id"))
			})
		})
	})
})

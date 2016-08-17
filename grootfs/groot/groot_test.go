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
		localCloner  *grootfakes.FakeCloner
		remoteCloner *grootfakes.FakeCloner
		bundler      *grootfakes.FakeBundler
		groot        *grootpkg.Groot
		bundle       *grootfakes.FakeBundle
		logger       lager.Logger
	)

	BeforeEach(func() {
		localCloner = new(grootfakes.FakeCloner)
		remoteCloner = new(grootfakes.FakeCloner)
		bundle = new(grootfakes.FakeBundle)
		bundler = new(grootfakes.FakeBundler)
		bundler.MakeBundleReturns(bundle, nil)

		logger = lagertest.NewTestLogger("groot")
		groot = grootpkg.IamGroot(bundler, localCloner, remoteCloner)
	})

	Describe("Create", func() {
		It("makes a bundle", func() {
			_, err := groot.Create(logger, grootpkg.CreateSpec{
				ID: "some-id",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(bundler.MakeBundleCallCount()).To(Equal(1))
			_, id := bundler.MakeBundleArgsForCall(0)
			Expect(id).To(Equal("some-id"))
		})

		It("returns the bundle path", func() {
			bundle.PathReturns("/path/to/bundle")

			bundle, err := groot.Create(logger, grootpkg.CreateSpec{})
			Expect(err).NotTo(HaveOccurred())
			Expect(bundle.Path()).To(Equal("/path/to/bundle"))
		})

		Context("when creating the bundle fails", func() {
			BeforeEach(func() {
				bundler.MakeBundleReturns(nil, errors.New("Failed to make bundle"))
			})

			It("returns the error", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{})
				Expect(err).To(MatchError("making bundle: Failed to make bundle"))
			})
		})

		Context("when using a local image", func() {
			It("clones the image", func() {
				bundle.PathReturns("/path/to/bundle")
				bundle.RootFSPathReturns("/path/to/bundle/rootfs")

				uidMappings := []grootpkg.IDMappingSpec{grootpkg.IDMappingSpec{HostID: 1, NamespaceID: 2, Size: 10}}
				gidMappings := []grootpkg.IDMappingSpec{grootpkg.IDMappingSpec{HostID: 10, NamespaceID: 20, Size: 100}}

				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image:       "/path/to/image",
					UIDMappings: uidMappings,
					GIDMappings: gidMappings,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(localCloner.CloneCallCount()).To(Equal(1))
				Expect(remoteCloner.CloneCallCount()).To(Equal(0))
				_, cloneSpec := localCloner.CloneArgsForCall(0)
				Expect(cloneSpec.Image).To(Equal("/path/to/image"))
				Expect(cloneSpec.RootFSPath).To(Equal("/path/to/bundle/rootfs"))
				Expect(cloneSpec.UIDMappings).To(Equal(uidMappings))
				Expect(cloneSpec.GIDMappings).To(Equal(gidMappings))
			})

			Context("when cloning fails", func() {
				BeforeEach(func() {
					localCloner.CloneReturns(errors.New("Failed to clone"))
				})

				It("returns the error", func() {
					_, err := groot.Create(logger, grootpkg.CreateSpec{
						Image: "/path/to/image",
					})
					Expect(err).To(MatchError("cloning: Failed to clone"))
				})

				It("deletes the bundle", func() {
					_, err := groot.Create(logger, grootpkg.CreateSpec{
						ID:    "some-id",
						Image: "/path/to/image",
					})
					Expect(err).To(HaveOccurred())

					Expect(bundler.DeleteBundleCallCount()).To(Equal(1))
					_, id := bundler.DeleteBundleArgsForCall(0)
					Expect(id).To(Equal("some-id"))
				})
			})
		})

		Context("when using a remote image", func() {
			It("clones the image", func() {
				bundle.PathReturns("/path/to/bundle")
				bundle.RootFSPathReturns("/path/to/bundle/rootfs")

				uidMappings := []grootpkg.IDMappingSpec{grootpkg.IDMappingSpec{HostID: 1, NamespaceID: 2, Size: 10}}
				gidMappings := []grootpkg.IDMappingSpec{grootpkg.IDMappingSpec{HostID: 10, NamespaceID: 20, Size: 100}}

				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image:       "docker:///path/to/image",
					UIDMappings: uidMappings,
					GIDMappings: gidMappings,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(localCloner.CloneCallCount()).To(Equal(0))
				Expect(remoteCloner.CloneCallCount()).To(Equal(1))
				_, cloneSpec := remoteCloner.CloneArgsForCall(0)
				Expect(cloneSpec.Image).To(Equal("docker:///path/to/image"))
				Expect(cloneSpec.RootFSPath).To(Equal("/path/to/bundle/rootfs"))
				Expect(cloneSpec.UIDMappings).To(Equal(uidMappings))
				Expect(cloneSpec.GIDMappings).To(Equal(gidMappings))
			})

			Context("when cloning fails", func() {
				BeforeEach(func() {
					remoteCloner.CloneReturns(errors.New("Failed to clone"))
				})

				It("returns the error", func() {
					_, err := groot.Create(logger, grootpkg.CreateSpec{
						Image: "docker:///path/to/image",
					})
					Expect(err).To(MatchError("cloning: Failed to clone"))
				})

				It("deletes the bundle", func() {
					_, err := groot.Create(logger, grootpkg.CreateSpec{
						ID:    "some-id",
						Image: "docker:///path/to/image",
					})
					Expect(err).To(HaveOccurred())

					Expect(bundler.DeleteBundleCallCount()).To(Equal(1))
					_, id := bundler.DeleteBundleArgsForCall(0)
					Expect(id).To(Equal("some-id"))
				})
			})
		})

		Context("when the image is not a valid URL", func() {
			It("returns an error", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image: "%%!!#@!^&",
				})
				Expect(err).To(MatchError(ContainSubstring("parsing image url")))
			})

			It("does not create a bundle", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image: "%%!!#@!^&",
				})
				Expect(err).To(HaveOccurred())

				Expect(bundler.MakeBundleCallCount()).To(Equal(0))
			})
		})
	})
})

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
		fakeBundle       *grootfakes.FakeBundle
		fakeBundler      *grootfakes.FakeBundler
		fakeLocalCloner  *grootfakes.FakeCloner
		fakeRemoteCloner *grootfakes.FakeCloner
		fakeVolumeDriver *grootfakes.FakeVolumeDriver

		groot  *grootpkg.Groot
		logger lager.Logger
	)

	BeforeEach(func() {
		fakeBundle = new(grootfakes.FakeBundle)
		fakeBundler = new(grootfakes.FakeBundler)
		fakeLocalCloner = new(grootfakes.FakeCloner)
		fakeRemoteCloner = new(grootfakes.FakeCloner)
		fakeVolumeDriver = new(grootfakes.FakeVolumeDriver)
		fakeBundler.MakeBundleReturns(fakeBundle, nil)

		logger = lagertest.NewTestLogger("groot")
		groot = grootpkg.IamGroot(fakeBundler, fakeLocalCloner, fakeRemoteCloner, fakeVolumeDriver)
	})

	Describe("Create", func() {
		It("makes a bundle", func() {
			_, err := groot.Create(logger, grootpkg.CreateSpec{
				ID: "some-id",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeBundler.MakeBundleCallCount()).To(Equal(1))
			_, id := fakeBundler.MakeBundleArgsForCall(0)
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
				fakeBundler.MakeBundleReturns(nil, errors.New("Failed to make bundle"))
			})

			It("returns the error", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{})
				Expect(err).To(MatchError("making bundle: Failed to make bundle"))
			})
		})

		Context("when using a local image", func() {
			It("clones the image", func() {
				fakeBundle.PathReturns("/path/to/bundle")
				fakeBundle.RootFSPathReturns("/path/to/bundle/rootfs")

				uidMappings := []grootpkg.IDMappingSpec{grootpkg.IDMappingSpec{HostID: 1, NamespaceID: 2, Size: 10}}
				gidMappings := []grootpkg.IDMappingSpec{grootpkg.IDMappingSpec{HostID: 10, NamespaceID: 20, Size: 100}}

				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image:       "/path/to/image",
					UIDMappings: uidMappings,
					GIDMappings: gidMappings,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeLocalCloner.CloneCallCount()).To(Equal(1))
				Expect(fakeRemoteCloner.CloneCallCount()).To(Equal(0))
				_, cloneSpec := fakeLocalCloner.CloneArgsForCall(0)
				Expect(cloneSpec.Image).To(Equal("/path/to/image"))
				Expect(cloneSpec.Bundle).To(Equal(fakeBundle))
				Expect(cloneSpec.UIDMappings).To(Equal(uidMappings))
				Expect(cloneSpec.GIDMappings).To(Equal(gidMappings))
			})

			Context("when cloning fails", func() {
				BeforeEach(func() {
					fakeLocalCloner.CloneReturns(errors.New("Failed to clone"))
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

					Expect(fakeBundler.DeleteBundleCallCount()).To(Equal(1))
					_, id := fakeBundler.DeleteBundleArgsForCall(0)
					Expect(id).To(Equal("some-id"))
				})
			})
		})

		Context("when using a remote image", func() {
			It("clones the image", func() {
				fakeBundle.PathReturns("/path/to/bundle")
				fakeBundle.RootFSPathReturns("/path/to/bundle/rootfs")

				uidMappings := []grootpkg.IDMappingSpec{grootpkg.IDMappingSpec{HostID: 1, NamespaceID: 2, Size: 10}}
				gidMappings := []grootpkg.IDMappingSpec{grootpkg.IDMappingSpec{HostID: 10, NamespaceID: 20, Size: 100}}

				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image:       "docker:///path/to/image",
					UIDMappings: uidMappings,
					GIDMappings: gidMappings,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeLocalCloner.CloneCallCount()).To(Equal(0))
				Expect(fakeRemoteCloner.CloneCallCount()).To(Equal(1))
				_, cloneSpec := fakeRemoteCloner.CloneArgsForCall(0)
				Expect(cloneSpec.Image).To(Equal("docker:///path/to/image"))
				Expect(cloneSpec.Bundle).To(Equal(fakeBundle))
				Expect(cloneSpec.UIDMappings).To(Equal(uidMappings))
				Expect(cloneSpec.GIDMappings).To(Equal(gidMappings))
			})

			Context("when cloning fails", func() {
				BeforeEach(func() {
					fakeRemoteCloner.CloneReturns(errors.New("Failed to clone"))
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

					Expect(fakeBundler.DeleteBundleCallCount()).To(Equal(1))
					_, id := fakeBundler.DeleteBundleArgsForCall(0)
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

				Expect(fakeBundler.MakeBundleCallCount()).To(Equal(0))
			})
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			fakeBundler.BundleReturns(fakeBundle)
			fakeBundle.RootFSPathReturns("/path/to/bundle/rootfs")
		})

		It("deletes a bundle", func() {
			Expect(groot.Delete(logger, "some-id")).To(Succeed())

			_, bundleId := fakeBundler.DeleteBundleArgsForCall(0)
			Expect(bundleId).To(Equal("some-id"))

			_, bundlePath := fakeVolumeDriver.DestroyArgsForCall(0)
			Expect(bundlePath).To(Equal(fakeBundle.RootFSPath()))
		})

		Context("when deleting a bundle fails", func() {
			BeforeEach(func() {
				fakeBundler.DeleteBundleReturns(errors.New("Boom!"))
			})

			It("returns an error", func() {
				err := groot.Delete(logger, "some-id")
				Expect(err).To(MatchError("deleting bundle: Boom!"))
			})
		})

		Context("when destroying the volume fails", func() {
			BeforeEach(func() {
				fakeVolumeDriver.DestroyReturns(errors.New("Boom!"))
			})

			It("returns an error", func() {
				err := groot.Delete(logger, "some-id")
				Expect(err).To(MatchError("Boom!"))
			})
		})
	})
})

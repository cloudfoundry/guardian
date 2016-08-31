package groot_test

import (
	"errors"
	"net/url"

	grootpkg "code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/groot/grootfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("I AM GROOT, the Orchestrator", func() {
	var (
		fakeBundler     *grootfakes.FakeBundler
		fakeImagePuller *grootfakes.FakeImagePuller
		groot           *grootpkg.Groot
		logger          lager.Logger
	)

	BeforeEach(func() {
		fakeBundler = new(grootfakes.FakeBundler)
		fakeImagePuller = new(grootfakes.FakeImagePuller)
		groot = grootpkg.IamGroot(fakeBundler, fakeImagePuller)
		logger = lagertest.NewTestLogger("groot")
	})

	Describe("Create", func() {
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

				Expect(fakeBundler.CreateCallCount()).To(Equal(0))
			})
		})

		Context("when the id already exists", func() {
			BeforeEach(func() {
				fakeBundler.ExistsReturns(true, nil)
			})

			It("returns an error", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image: "/path/to/image",
					ID:    "some-id",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeBundler.CreateCallCount()).To(Equal(0))
				Expect(err).To(MatchError(ContainSubstring("bundle for id `some-id` already exists")))
			})

			It("does not pull the image", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image: "/path/to/image",
					ID:    "some-id",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeImagePuller.PullCallCount()).To(Equal(0))
				Expect(err).To(MatchError(ContainSubstring("bundle for id `some-id` already exists")))
			})
		})

		Context("when checking if the id exists fails", func() {
			BeforeEach(func() {
				fakeBundler.ExistsReturns(false, errors.New("Checking if the bundle ID exists"))
			})

			It("returns an error", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image: "/path/to/image",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeBundler.CreateCallCount()).To(Equal(0))
				Expect(err).To(MatchError(ContainSubstring("Checking if the bundle ID exists")))
			})

			It("does not pull the image", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image: "/path/to/image",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeImagePuller.PullCallCount()).To(Equal(0))
				Expect(err).To(MatchError(ContainSubstring("Checking if the bundle ID exists")))
			})
		})

		It("pulls the image", func() {
			uidMappings := []grootpkg.IDMappingSpec{grootpkg.IDMappingSpec{HostID: 1, NamespaceID: 2, Size: 10}}
			gidMappings := []grootpkg.IDMappingSpec{grootpkg.IDMappingSpec{HostID: 10, NamespaceID: 20, Size: 100}}

			_, err := groot.Create(logger, grootpkg.CreateSpec{
				Image:       "/path/to/image",
				UIDMappings: uidMappings,
				GIDMappings: gidMappings,
			})
			Expect(err).NotTo(HaveOccurred())

			imageURL, err := url.Parse("/path/to/image")
			Expect(err).NotTo(HaveOccurred())
			_, imageSpec := fakeImagePuller.PullArgsForCall(0)
			Expect(imageSpec.ImageSrc).To(Equal(imageURL))
			Expect(imageSpec.UIDMappings).To(Equal(uidMappings))
			Expect(imageSpec.GIDMappings).To(Equal(gidMappings))
		})

		Context("when pulling the image fails", func() {
			BeforeEach(func() {
				fakeImagePuller.PullReturns(grootpkg.BundleSpec{}, errors.New("Failed to pull image"))
			})

			It("returns the error", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image: "/path/to/image",
				})
				Expect(err).To(MatchError("pulling the image: Failed to pull image"))
			})

			It("does not create a bundle", func() {
				groot.Create(logger, grootpkg.CreateSpec{
					Image: "/path/to/image",
				})
				Expect(fakeBundler.CreateCallCount()).To(Equal(0))
			})
		})

		It("makes a bundle", func() {
			bundleSpec := grootpkg.BundleSpec{
				VolumePath: "/path/to/volume",
				Image: specsv1.Image{
					Author: "Groot",
				},
			}
			fakeImagePuller.PullReturns(bundleSpec, nil)

			_, err := groot.Create(logger, grootpkg.CreateSpec{
				ID:    "some-id",
				Image: "/path/to/image",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeBundler.CreateCallCount()).To(Equal(1))
			_, id, createBundlerSpec := fakeBundler.CreateArgsForCall(0)
			Expect(id).To(Equal("some-id"))
			Expect(createBundlerSpec).To(Equal(bundleSpec))
		})

		It("returns the bundle path", func() {
			fakeBundle := new(grootfakes.FakeBundle)
			fakeBundle.PathReturns("/path/to/bundle")
			fakeBundler.CreateReturns(fakeBundle, nil)

			bundle, err := groot.Create(logger, grootpkg.CreateSpec{})
			Expect(err).NotTo(HaveOccurred())
			Expect(bundle.Path()).To(Equal("/path/to/bundle"))
		})

		Context("when creating the bundle fails", func() {
			BeforeEach(func() {
				fakeBundler.CreateReturns(nil, errors.New("Failed to make bundle"))
			})

			It("returns the error", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{})
				Expect(err).To(MatchError("making bundle: Failed to make bundle"))
			})
		})

		Context("when disk limit is given", func() {
			It("applies the disk limit", func() {
				bundleSpec := grootpkg.BundleSpec{
					DiskLimit:  int64(1024),
					VolumePath: "/path/to/volume",
					Image: specsv1.Image{
						Author: "Groot",
					},
				}
				fakeImagePuller.PullReturns(bundleSpec, nil)

				_, err := groot.Create(logger, grootpkg.CreateSpec{
					ID:        "some-id",
					DiskLimit: int64(1024),
					Image:     "/path/to/image",
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBundler.CreateCallCount()).To(Equal(1))
				_, id, createBundlerSpec := fakeBundler.CreateArgsForCall(0)
				Expect(id).To(Equal("some-id"))
				Expect(createBundlerSpec).To(Equal(bundleSpec))
			})
		})
	})

	Describe("Delete", func() {
		It("destroys a bundle", func() {
			Expect(groot.Delete(logger, "some-id")).To(Succeed())

			_, bundleId := fakeBundler.DestroyArgsForCall(0)
			Expect(bundleId).To(Equal("some-id"))
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

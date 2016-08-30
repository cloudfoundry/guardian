package volplugin_test

import (
	"errors"
	"net/url"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden-shed/rootfs_provider"
	"code.cloudfoundry.org/guardian/gardener/gardenerfakes"
	"code.cloudfoundry.org/guardian/volplugin"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("CompositeVolumeCreator", func() {
	var (
		fakePM           *gardenerfakes.FakePropertyManager
		fakeGrootfsVC    *gardenerfakes.FakeVolumeCreator
		fakeGardenShedVC *gardenerfakes.FakeVolumeCreator
		logger           *lagertest.TestLogger

		compositeVolumeCreator *volplugin.CompositeVolumeCreator
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("composite-volume-creator")
		fakeGrootfsVC = new(gardenerfakes.FakeVolumeCreator)
		fakeGardenShedVC = new(gardenerfakes.FakeVolumeCreator)
		fakePM = new(gardenerfakes.FakePropertyManager)
		compositeVolumeCreator = volplugin.NewCompositeVolumeCreator(fakeGrootfsVC, fakeGardenShedVC, fakePM)
	})

	Describe("Create", func() {
		var rootfs *url.URL

		Context("when it's not a groofs url", func() {
			BeforeEach(func() {
				var err error
				rootfs, err = url.Parse("http://hello.com")
				Expect(err).ToNot(HaveOccurred())
			})

			It("uses the garden-shed volume creator", func() {
				_, _, err := compositeVolumeCreator.Create(logger, "image-id", rootfs_provider.Spec{RootFS: rootfs})
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeGrootfsVC.CreateCallCount()).To(Equal(0))
				Expect(fakeGardenShedVC.CreateCallCount()).To(Equal(1))

				_, handler, spec := fakeGardenShedVC.CreateArgsForCall(0)
				Expect(handler).To(Equal("image-id"))
				Expect(spec).To(Equal(rootfs_provider.Spec{RootFS: rootfs}))
			})

			It("returns the same returns of the garden-shed volume creator", func() {
				expectedRootFSPathReturn := "/some/place/here"
				expectedEnvsReturn := []string{"HELLO", "THERE"}
				expectedErrorReturn := errors.New("sorry!")
				fakeGardenShedVC.CreateReturns(expectedRootFSPathReturn, expectedEnvsReturn, expectedErrorReturn)

				rootfsPath, envs, err := compositeVolumeCreator.Create(logger, "image-id", rootfs_provider.Spec{RootFS: rootfs})
				Expect(rootfsPath).To(Equal(expectedRootFSPathReturn))
				Expect(envs).To(Equal(expectedEnvsReturn))
				Expect(err).To(Equal(expectedErrorReturn))
			})

			It("writes to the property manager the volumePlugin property", func() {
				_, _, err := compositeVolumeCreator.Create(logger, "image-id", rootfs_provider.Spec{RootFS: rootfs})
				Expect(err).ToNot(HaveOccurred())

				Expect(fakePM.SetCallCount()).To(Equal(1))
				handle, property, value := fakePM.SetArgsForCall(0)

				Expect(handle).To(Equal("image-id"))
				Expect(property).To(Equal("volumePlugin"))
				Expect(value).To(Equal("garden-shed"))
			})

			It("uses the garden-shed volume creator to GC", func() {
				_, _, err := compositeVolumeCreator.Create(logger, "image-id", rootfs_provider.Spec{RootFS: rootfs})
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeGrootfsVC.GCCallCount()).To(Equal(0))
				Expect(fakeGardenShedVC.GCCallCount()).To(Equal(1))
			})

			Context("when calling the GC fails", func() {
				BeforeEach(func() {
					fakeGardenShedVC.GCReturns(errors.New("failed to GC"))
				})

				It("doesn't fail, but logs the error", func() {
					_, _, err := compositeVolumeCreator.Create(logger, "image-id", rootfs_provider.Spec{RootFS: rootfs})
					Expect(err).ToNot(HaveOccurred())
					Expect(logger.TestSink.Buffer()).To(gbytes.Say("failed to GC"))
				})
			})
		})

		Context("when it's a groofs url", func() {
			BeforeEach(func() {
				var err error
				rootfs, err = url.Parse("grootfs+docker:///hello.com")
				Expect(err).ToNot(HaveOccurred())
			})

			It("uses the grootfs volume creator", func() {
				_, _, err := compositeVolumeCreator.Create(logger, "image-id", rootfs_provider.Spec{RootFS: rootfs})
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeGardenShedVC.CreateCallCount()).To(Equal(0))
				Expect(fakeGrootfsVC.CreateCallCount()).To(Equal(1))
			})

			It("returns the same returns of the grootfs volume creator", func() {
				expectedRootFSPathReturn := "/some/place/here"
				expectedEnvsReturn := []string{"HELLO", "THERE"}
				expectedErrorReturn := errors.New("sorry!")
				fakeGrootfsVC.CreateReturns(expectedRootFSPathReturn, expectedEnvsReturn, expectedErrorReturn)

				rootfsPath, envs, err := compositeVolumeCreator.Create(logger, "image-id", rootfs_provider.Spec{RootFS: rootfs})
				Expect(rootfsPath).To(Equal(expectedRootFSPathReturn))
				Expect(envs).To(Equal(expectedEnvsReturn))
				Expect(err).To(Equal(expectedErrorReturn))
			})

			It("removes the `grootfs+` part of the given rootfs", func() {
				_, _, err := compositeVolumeCreator.Create(logger, "image-id", rootfs_provider.Spec{RootFS: rootfs})
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeGrootfsVC.CreateCallCount()).To(Equal(1))
				_, handler, spec := fakeGrootfsVC.CreateArgsForCall(0)
				Expect(handler).To(Equal("image-id"))

				expectedRootFS, err := url.Parse("docker:///hello.com")
				Expect(err).ToNot(HaveOccurred())
				Expect(spec).To(Equal(rootfs_provider.Spec{RootFS: expectedRootFS}))
			})

			It("writes to the property manager the volumePlugin property", func() {
				_, _, err := compositeVolumeCreator.Create(logger, "image-id", rootfs_provider.Spec{RootFS: rootfs})
				Expect(err).ToNot(HaveOccurred())

				Expect(fakePM.SetCallCount()).To(Equal(1))
				handle, property, value := fakePM.SetArgsForCall(0)

				Expect(handle).To(Equal("image-id"))
				Expect(property).To(Equal("volumePlugin"))
				Expect(value).To(Equal("grootfs"))
			})

			It("uses the grootfs volume creator to GC", func() {
				_, _, err := compositeVolumeCreator.Create(logger, "image-id", rootfs_provider.Spec{RootFS: rootfs})
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeGardenShedVC.GCCallCount()).To(Equal(0))
				Expect(fakeGrootfsVC.GCCallCount()).To(Equal(1))
			})

			Context("when calling the GC fails", func() {
				BeforeEach(func() {
					fakeGrootfsVC.GCReturns(errors.New("failed to GC"))
				})

				It("doesn't fail, but logs the error", func() {
					_, _, err := compositeVolumeCreator.Create(logger, "image-id", rootfs_provider.Spec{RootFS: rootfs})
					Expect(err).ToNot(HaveOccurred())
					Expect(logger.TestSink.Buffer()).To(gbytes.Say("failed to GC"))
				})
			})
		})
	})

	Describe("Destroy", func() {
		BeforeEach(func() {
			fakePM.GetStub = func(handle string, name string) (string, bool) {
				if handle == "grootfs-image-id" {
					if name == "volumePlugin" {
						return "grootfs", true
					}
				}

				return "", false
			}
		})
		Context("when the property manager doesn't tell it's a grootfs image", func() {
			It("uses the garden-shed volume creator", func() {
				err := compositeVolumeCreator.Destroy(logger, "image-id")
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeGrootfsVC.DestroyCallCount()).To(Equal(0))
				Expect(fakeGardenShedVC.DestroyCallCount()).To(Equal(1))

				_, handler := fakeGardenShedVC.DestroyArgsForCall(0)
				Expect(handler).To(Equal("image-id"))
			})

			Context("when garden-shed volume creator fails", func() {
				BeforeEach(func() {
					fakeGardenShedVC.DestroyReturns(errors.New("failed!"))
				})

				It("fails with the same error", func() {
					err := compositeVolumeCreator.Destroy(logger, "image-id")
					Expect(err).To(MatchError("failed!"))
				})
			})
		})

		Context("when the property manager tells it's a grootfs image", func() {
			It("uses the grootfs volume creator", func() {
				err := compositeVolumeCreator.Destroy(logger, "grootfs-image-id")
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeGardenShedVC.DestroyCallCount()).To(Equal(0))
				Expect(fakeGrootfsVC.DestroyCallCount()).To(Equal(1))

				_, handler := fakeGrootfsVC.DestroyArgsForCall(0)
				Expect(handler).To(Equal("grootfs-image-id"))
			})

			Context("when grootfs volume creator fails", func() {
				BeforeEach(func() {
					fakeGrootfsVC.DestroyReturns(errors.New("failed!"))
				})

				It("fails with the same error", func() {
					err := compositeVolumeCreator.Destroy(logger, "grootfs-image-id")
					Expect(err).To(MatchError("failed!"))
				})
			})
		})
	})

	Describe("Metrics", func() {
		BeforeEach(func() {
			fakePM.GetStub = func(handle string, name string) (string, bool) {
				if handle == "grootfs-image-id" {
					if name == "volumePlugin" {
						return "grootfs", true
					}
				}

				return "", false
			}
		})
		Context("when the property manager doesn't tell it's a grootfs image", func() {
			It("uses the garden-shed volume creator", func() {
				_, err := compositeVolumeCreator.Metrics(logger, "image-id")
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeGrootfsVC.MetricsCallCount()).To(Equal(0))
				Expect(fakeGardenShedVC.MetricsCallCount()).To(Equal(1))

				_, handler := fakeGardenShedVC.MetricsArgsForCall(0)
				Expect(handler).To(Equal("image-id"))
			})

			It("returns the whatever garden-shed volume creator returns", func() {
				expectedContainerDiskStatReturn := garden.ContainerDiskStat{
					TotalBytesUsed: 666,
				}
				expectedErrorReturn := errors.New("crash")
				fakeGardenShedVC.MetricsReturns(expectedContainerDiskStatReturn, expectedErrorReturn)

				diskStat, err := compositeVolumeCreator.Metrics(logger, "image-id")
				Expect(diskStat).To(Equal(expectedContainerDiskStatReturn))
				Expect(err).To(MatchError(expectedErrorReturn))
			})
		})

		Context("when the property manager tells it's a grootfs image", func() {
			It("uses the grootfs volume creator", func() {
				_, err := compositeVolumeCreator.Metrics(logger, "grootfs-image-id")
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeGardenShedVC.MetricsCallCount()).To(Equal(0))
				Expect(fakeGrootfsVC.MetricsCallCount()).To(Equal(1))

				_, handler := fakeGrootfsVC.MetricsArgsForCall(0)
				Expect(handler).To(Equal("grootfs-image-id"))
			})

			It("returns the whatever grootfs volume creator returns", func() {
				expectedContainerDiskStatReturn := garden.ContainerDiskStat{
					TotalBytesUsed: 666,
				}
				expectedErrorReturn := errors.New("crash")
				fakeGrootfsVC.MetricsReturns(expectedContainerDiskStatReturn, expectedErrorReturn)

				diskStat, err := compositeVolumeCreator.Metrics(logger, "grootfs-image-id")
				Expect(diskStat).To(Equal(expectedContainerDiskStatReturn))
				Expect(err).To(MatchError(expectedErrorReturn))
			})
		})
	})

	Describe("GC", func() {
		It("uses the garden-shed volume creator", func() {
			Expect(compositeVolumeCreator.GC(logger)).To(Succeed())

			Expect(fakeGrootfsVC.GCCallCount()).To(Equal(0))
			Expect(fakeGardenShedVC.GCCallCount()).To(Equal(1))
		})

		It("returns the same error that the garden-shed volume creator returns", func() {
			fakeGardenShedVC.GCReturns(errors.New("garden-shed error"))

			err := compositeVolumeCreator.GC(logger)
			Expect(err).To(MatchError("garden-shed error"))
		})
	})
})

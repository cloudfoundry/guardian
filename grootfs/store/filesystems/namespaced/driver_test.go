package namespaced_test

import (
	"errors"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/groot/grootfakes"
	"code.cloudfoundry.org/grootfs/store/filesystems/namespaced"
	"code.cloudfoundry.org/grootfs/store/filesystems/namespaced/namespacedfakes"
	"code.cloudfoundry.org/grootfs/store/image_manager"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Driver", func() {
	var (
		internalDriver    *namespacedfakes.FakeInternalDriver
		driver            *namespaced.Driver
		logger            lager.Logger
		reexecer          *grootfakes.FakeSandboxReexecer
		shouldCloneUserNs bool
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("driver")
		internalDriver = new(namespacedfakes.FakeInternalDriver)
		reexecer = new(grootfakes.FakeSandboxReexecer)
		shouldCloneUserNs = false
	})

	JustBeforeEach(func() {
		driver = namespaced.New(internalDriver, reexecer, shouldCloneUserNs)
	})

	Describe("VolumePath", func() {
		JustBeforeEach(func() {
			internalDriver.VolumePathReturns("abc", errors.New("error"))
		})

		It("decorates the internal driver function", func() {
			path, err := driver.VolumePath(logger, "123")
			Expect(path).To(Equal("abc"))
			Expect(err).To(MatchError("error"))
			Expect(internalDriver.VolumePathCallCount()).To(Equal(1))
			loggerArg, id := internalDriver.VolumePathArgsForCall(0)
			Expect(loggerArg).To(Equal(logger))
			Expect(id).To(Equal("123"))
		})
	})

	Describe("CreateVolume", func() {
		JustBeforeEach(func() {
			internalDriver.CreateVolumeReturns("abc", errors.New("error"))
		})

		It("decorates the internal driver function", func() {
			path, err := driver.CreateVolume(logger, "123", "456")
			Expect(path).To(Equal("abc"))
			Expect(err).To(MatchError("error"))
			Expect(internalDriver.CreateVolumeCallCount()).To(Equal(1))
			loggerArg, parentId, id := internalDriver.CreateVolumeArgsForCall(0)
			Expect(loggerArg).To(Equal(logger))
			Expect(parentId).To(Equal("123"))
			Expect(id).To(Equal("456"))
		})
	})

	Describe("DestroyVolume", func() {
		JustBeforeEach(func() {
			internalDriver.MarshalReturns([]byte(`{"super-cool":"json"}`), nil)
			internalDriver.DestroyVolumeReturns(errors.New("error"))
		})

		It("does not reexec", func() {
			driver.DestroyVolume(logger, "123")
			Expect(reexecer.ReexecCallCount()).To(BeZero())
		})

		It("decorates the internal driver function", func() {
			err := driver.DestroyVolume(logger, "123")
			Expect(err).To(MatchError("error"))
			Expect(internalDriver.DestroyVolumeCallCount()).To(Equal(1))
			loggerArg, id := internalDriver.DestroyVolumeArgsForCall(0)
			Expect(loggerArg).To(Equal(logger))
			Expect(id).To(Equal("123"))
		})

		Context("when asked to clone user namespace", func() {
			BeforeEach(func() {
				shouldCloneUserNs = true
			})

			It("reexecs into new user ns", func() {
				Expect(driver.DestroyVolume(logger, "123")).To(Succeed())
				Expect(reexecer.ReexecCallCount()).To(Equal(1))
				reexecCmd, reexecSpec := reexecer.ReexecArgsForCall(0)
				Expect(reexecCmd).To(Equal("destroy-volume"))
				Expect(reexecSpec.CloneUserns).To(BeTrue())
			})
		})
	})

	Describe("Volumes", func() {
		JustBeforeEach(func() {
			internalDriver.VolumesReturns([]string{"abc"}, errors.New("error"))
		})

		It("decorates the internal driver function", func() {
			paths, err := driver.Volumes(logger)
			Expect(paths).To(Equal([]string{"abc"}))
			Expect(err).To(MatchError("error"))
			Expect(internalDriver.VolumesCallCount()).To(Equal(1))
			loggerArg := internalDriver.VolumesArgsForCall(0)
			Expect(loggerArg).To(Equal(logger))
		})
	})

	Describe("MoveVolume", func() {
		JustBeforeEach(func() {
			internalDriver.MoveVolumeReturns(errors.New("error"))
		})

		It("decorates the internal driver function", func() {
			err := driver.MoveVolume(logger, "123", "456")
			Expect(err).To(MatchError("error"))
			Expect(internalDriver.MoveVolumeCallCount()).To(Equal(1))
			loggerArg, from, to := internalDriver.MoveVolumeArgsForCall(0)
			Expect(loggerArg).To(Equal(logger))
			Expect(from).To(Equal("123"))
			Expect(to).To(Equal("456"))
		})
	})

	Describe("WriteVolumeMeta", func() {
		JustBeforeEach(func() {
			internalDriver.WriteVolumeMetaReturns(errors.New("error"))
		})

		It("decorates the internal driver function", func() {
			volMeta := base_image_puller.VolumeMeta{Size: 1000}
			err := driver.WriteVolumeMeta(logger, "123", volMeta)
			Expect(err).To(MatchError("error"))
			Expect(internalDriver.WriteVolumeMetaCallCount()).To(Equal(1))
			loggerArg, id, volMetaArgs := internalDriver.WriteVolumeMetaArgsForCall(0)
			Expect(loggerArg).To(Equal(logger))
			Expect(id).To(Equal("123"))
			Expect(volMetaArgs).To(Equal(volMeta))
		})
	})

	Describe("HandleOpaqueWhiteouts", func() {
		JustBeforeEach(func() {
			internalDriver.HandleOpaqueWhiteoutsReturns(errors.New("error"))
		})

		It("decorates the internal driver function", func() {
			err := driver.HandleOpaqueWhiteouts(logger, "123", []string{"456"})
			Expect(err).To(MatchError("error"))
			Expect(internalDriver.HandleOpaqueWhiteoutsCallCount()).To(Equal(1))
			loggerArg, id, opaques := internalDriver.HandleOpaqueWhiteoutsArgsForCall(0)
			Expect(loggerArg).To(Equal(logger))
			Expect(id).To(Equal("123"))
			Expect(opaques).To(Equal([]string{"456"}))
		})
	})

	Describe("CreateImage", func() {
		JustBeforeEach(func() {
			internalDriver.CreateImageReturns(groot.MountInfo{Destination: "Dimension 31-C"}, errors.New("error"))
		})

		It("decorates the internal driver function", func() {
			mountInfo, err := driver.CreateImage(logger, image_manager.ImageDriverSpec{Mount: true})
			Expect(mountInfo).To(Equal(groot.MountInfo{Destination: "Dimension 31-C"}))
			Expect(err).To(MatchError("error"))
			Expect(internalDriver.CreateImageCallCount()).To(Equal(1))
			loggerArg, specArg := internalDriver.CreateImageArgsForCall(0)
			Expect(loggerArg).To(Equal(logger))
			Expect(specArg).To(Equal(image_manager.ImageDriverSpec{Mount: true}))
		})
	})

	Describe("DestroyImage", func() {
		JustBeforeEach(func() {
			internalDriver.MarshalReturns([]byte(`{"super-cool":"json"}`), nil)
			internalDriver.DestroyImageReturns(errors.New("error"))
		})

		It("decorates the internal driver function", func() {
			err := driver.DestroyImage(logger, "123")
			Expect(err).To(MatchError("error"))
			Expect(internalDriver.DestroyImageCallCount()).To(Equal(1))
			loggerArg, id := internalDriver.DestroyImageArgsForCall(0)
			Expect(loggerArg).To(Equal(logger))
			Expect(id).To(Equal("123"))
		})

		It("does not reexec", func() {
			driver.DestroyImage(logger, "123")
			Expect(reexecer.ReexecCallCount()).To(Equal(0))
		})

		Context("when asked to clone user namespace", func() {
			BeforeEach(func() {
				shouldCloneUserNs = true
			})

			It("reexecs into new user ns", func() {
				Expect(driver.DestroyImage(logger, "123")).To(Succeed())
				Expect(reexecer.ReexecCallCount()).To(Equal(1))
				reexecCmd, reexecSpec := reexecer.ReexecArgsForCall(0)
				Expect(reexecCmd).To(Equal("destroy-image"))
				Expect(reexecSpec.CloneUserns).To(BeTrue())
			})
		})
	})

	Describe("FetchStats", func() {
		JustBeforeEach(func() {
			internalDriver.FetchStatsReturns(groot.VolumeStats{DiskUsage: groot.DiskUsage{TotalBytesUsed: 100}}, errors.New("error"))
		})

		It("decorates the internal driver function", func() {
			stats, err := driver.FetchStats(logger, "id-1")
			Expect(stats).To(Equal(groot.VolumeStats{DiskUsage: groot.DiskUsage{TotalBytesUsed: 100}}))
			Expect(err).To(MatchError("error"))
			Expect(internalDriver.FetchStatsCallCount()).To(Equal(1))
			loggerArg, imageIdArg := internalDriver.FetchStatsArgsForCall(0)
			Expect(loggerArg).To(Equal(logger))
			Expect(imageIdArg).To(Equal("id-1"))
		})
	})
})

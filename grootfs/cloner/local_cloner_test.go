package cloner_test

import (
	"errors"
	"io/ioutil"
	"os"

	clonerpkg "code.cloudfoundry.org/grootfs/cloner"
	"code.cloudfoundry.org/grootfs/cloner/clonerfakes"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LocalCloner", func() {
	var (
		streamer   *clonerfakes.FakeStreamer
		unpacker   *clonerfakes.FakeUnpacker
		volDriver  *clonerfakes.FakeVolumeDriver
		cloner     *clonerpkg.LocalCloner
		logger     lager.Logger
		volumePath string
		imagePath  string
	)

	BeforeEach(func() {
		var err error
		imagePath, err = ioutil.TempDir("", "image-path")
		Expect(err).ToNot(HaveOccurred())
		volumePath = "/path/to/cached/volume"
		streamer = new(clonerfakes.FakeStreamer)
		volDriver = new(clonerfakes.FakeVolumeDriver)
		unpacker = new(clonerfakes.FakeUnpacker)
		cloner = clonerpkg.NewLocalCloner(streamer, unpacker, volDriver)
		logger = lagertest.NewTestLogger("cloner")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(imagePath)).To(Succeed())
	})

	Describe("Clone", func() {

		It("snapshots the volume path to the rootfs path", func() {
			Expect(cloner.Clone(logger, groot.CloneSpec{
				Image:      imagePath,
				RootFSPath: "/rootfs-destination",
			})).To(Succeed())

			Expect(volDriver.SnapshotCallCount()).To(Equal(1))

			_, volumeID, rootfsPath := volDriver.SnapshotArgsForCall(0)

			calculatedVolID := integration.ImagePathToVolumeID(imagePath)
			Expect(volumeID).To(Equal(calculatedVolID))
			Expect(rootfsPath).To(Equal("/rootfs-destination"))
		})

		Context("when the image path doesn't exist", func() {
			It("returns an error", func() {
				err := cloner.Clone(logger, groot.CloneSpec{Image: "/not-here/sorry"})
				Expect(err).To(MatchError(ContainSubstring("checking image source")))
			})
		})

		Context("when snapshotting fails", func() {
			BeforeEach(func() {
				volDriver.SnapshotReturns(errors.New("failed"))
			})

			It("returns an error", func() {
				err := cloner.Clone(logger, groot.CloneSpec{Image: imagePath})
				Expect(err).To(MatchError(ContainSubstring("failed")))
			})
		})

		Context("when the volume is not cached", func() {
			BeforeEach(func() {
				volDriver.PathReturns("", errors.New("not-here"))
				volDriver.CreateReturns(volumePath, nil)
			})

			It("creates a new volume using sha of the image path + its timestamp", func() {
				Expect(cloner.Clone(logger, groot.CloneSpec{
					Image: imagePath,
				})).To(Succeed())

				Expect(volDriver.CreateCallCount()).To(Equal(1))

				calculatedVolID := integration.ImagePathToVolumeID(imagePath)
				_, parentID, volumeID := volDriver.CreateArgsForCall(0)
				Expect(parentID).To(BeEmpty())
				Expect(volumeID).To(Equal(calculatedVolID))
			})

			It("reads the correct source", func() {
				Expect(cloner.Clone(logger, groot.CloneSpec{
					Image: imagePath,
				})).To(Succeed())

				Expect(streamer.StreamCallCount()).To(Equal(1))
				_, source := streamer.StreamArgsForCall(0)
				Expect(source).To(Equal(imagePath))
			})

			It("writes using the correct write spec", func() {
				uidMappings := []groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: 1, NamespaceID: 2, Size: 10},
				}
				gidMappings := []groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: 10, NamespaceID: 20, Size: 100},
				}

				Expect(cloner.Clone(logger, groot.CloneSpec{
					Image:       imagePath,
					RootFSPath:  "/someplace",
					UIDMappings: uidMappings,
					GIDMappings: gidMappings,
				})).To(Succeed())

				Expect(unpacker.UnpackCallCount()).To(Equal(1))
				_, writeSpec := unpacker.UnpackArgsForCall(0)
				Expect(writeSpec.TargetPath).To(Equal(volumePath))
				Expect(writeSpec.UIDMappings).To(Equal(uidMappings))
				Expect(writeSpec.GIDMappings).To(Equal(gidMappings))
			})

			It("pipes the streamer and the unpacker", func() {
				pipeR, pipeW, err := os.Pipe()
				Expect(err).ToNot(HaveOccurred())

				streamer.StreamReturns(pipeR, 0, nil)

				Expect(cloner.Clone(logger, groot.CloneSpec{Image: imagePath})).To(Succeed())

				Expect(unpacker.UnpackCallCount()).To(Equal(1))
				_, writeSpec := unpacker.UnpackArgsForCall(0)

				_, err = pipeW.WriteString("hello-world")
				Expect(err).ToNot(HaveOccurred())
				Expect(pipeW.Close()).To(Succeed())

				contents, err := ioutil.ReadAll(writeSpec.Stream)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(contents)).To(Equal("hello-world"))
			})

			Context("when creating a volume fails", func() {
				BeforeEach(func() {
					volDriver.CreateReturns("", errors.New("failed"))
				})

				It("returns an error", func() {
					err := cloner.Clone(logger, groot.CloneSpec{
						Image: imagePath,
					})

					Expect(err).To(MatchError(ContainSubstring("failed")))
				})
			})

			Context("when the streamer fails", func() {
				It("returns an error", func() {
					streamer.StreamReturns(nil, 0, errors.New("cannot read"))

					Expect(
						cloner.Clone(logger, groot.CloneSpec{Image: imagePath}),
					).To(MatchError(ContainSubstring("cannot read")))
				})
			})

			Context("when the unpacker fails", func() {
				It("returns an error", func() {
					unpacker.UnpackReturns(errors.New("cannot write"))

					Expect(
						cloner.Clone(logger, groot.CloneSpec{Image: imagePath}),
					).To(MatchError(ContainSubstring("cannot write")))
				})
			})
		})

		Context("when the volume is cached", func() {
			BeforeEach(func() {
				volDriver.PathReturns(volumePath, nil)
			})

			It("doesn't create a new volume", func() {
				Expect(cloner.Clone(logger, groot.CloneSpec{
					Image: imagePath,
				})).To(Succeed())

				Expect(volDriver.CreateCallCount()).To(BeZero())
			})

			It("doesn't stream", func() {
				Expect(cloner.Clone(logger, groot.CloneSpec{
					Image: imagePath,
				})).To(Succeed())

				Expect(streamer.StreamCallCount()).To(BeZero())
			})

			It("doesn't unpack", func() {
				Expect(cloner.Clone(logger, groot.CloneSpec{
					Image: imagePath,
				})).To(Succeed())

				Expect(unpacker.UnpackCallCount()).To(BeZero())
			})
		})

	})
})

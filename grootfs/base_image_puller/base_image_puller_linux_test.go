package base_image_puller_test

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/base_image_puller/base_image_pullerfakes"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/groot/grootfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("Base Image Puller", func() {
	var (
		logger             lager.Logger
		fakeFetcher        *base_image_pullerfakes.FakeFetcher
		fakeUnpacker       *base_image_pullerfakes.FakeUnpacker
		fakeVolumeDriver   *base_image_pullerfakes.FakeVolumeDriver
		fakeLocksmith      *grootfakes.FakeLocksmith
		fakeMetricsEmitter *grootfakes.FakeMetricsEmitter
		fakeBaseDirHandler *base_image_pullerfakes.FakeBaseDirHandler
		expectedImgDesc    specsv1.Image

		baseImagePuller *base_image_puller.BaseImagePuller
		layerInfos      []groot.LayerInfo
		baseImageInfo   groot.BaseImageInfo

		tmpVolumesDir string
	)

	BeforeEach(func() {
		fakeUnpacker = new(base_image_pullerfakes.FakeUnpacker)

		fakeLocksmith = new(grootfakes.FakeLocksmith)
		fakeMetricsEmitter = new(grootfakes.FakeMetricsEmitter)
		fakeFetcher = new(base_image_pullerfakes.FakeFetcher)
		fakeBaseDirHandler = new(base_image_pullerfakes.FakeBaseDirHandler)

		expectedImgDesc = specsv1.Image{Author: "Groot"}
		layerInfos = []groot.LayerInfo{
			{BlobID: "i-am-a-layer", ChainID: "layer-111", ParentChainID: ""},
			{BlobID: "i-am-another-layer", ChainID: "chain-222", ParentChainID: "layer-111"},
			{BlobID: "i-am-the-last-layer", ChainID: "chain-333", ParentChainID: "chain-222"},
		}
		baseImageInfo = groot.BaseImageInfo{
			LayerInfos: layerInfos,
			Config:     expectedImgDesc,
		}
		fakeFetcher.BaseImageInfoReturns(baseImageInfo, nil)

		fakeFetcher.StreamBlobStub = func(_ lager.Logger, layerInfo groot.LayerInfo) (io.ReadCloser, int64, error) {
			buffer := bytes.NewBuffer([]byte{})
			stream := gzip.NewWriter(buffer)
			defer stream.Close()
			return ioutil.NopCloser(buffer), 0, nil
		}

		var err error
		tmpVolumesDir, err = ioutil.TempDir("", "volumes")
		Expect(err).NotTo(HaveOccurred())

		fakeVolumeDriver = new(base_image_pullerfakes.FakeVolumeDriver)
		fakeVolumeDriver.VolumePathStub = func(_ lager.Logger, id string) (string, error) {
			volumeDir := filepath.Join(tmpVolumesDir, id)
			_, err := os.Stat(volumeDir)
			return volumeDir, err
		}
		fakeVolumeDriver.CreateVolumeStub = func(_ lager.Logger, _, id string) (string, error) {
			volumeDir := filepath.Join(tmpVolumesDir, id)
			Expect(os.MkdirAll(volumeDir, 0777)).To(Succeed())
			return volumeDir, nil
		}
		fakeVolumeDriver.MoveVolumeStub = func(_ lager.Logger, from, to string) error {
			return os.Rename(from, to)
		}

		baseImagePuller = base_image_puller.NewBaseImagePuller(fakeFetcher, fakeUnpacker, fakeVolumeDriver, fakeMetricsEmitter, fakeLocksmith, fakeBaseDirHandler)
		logger = lagertest.NewTestLogger("image-puller")
	})

	Describe("FetchBaseImageInfo", func() {
		It("returns the image description", func() {
			baseImage, err := baseImagePuller.FetchBaseImageInfo(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(baseImage.Config).To(Equal(expectedImgDesc))
		})

		It("returns the chain ids", func() {
			baseImage, err := baseImagePuller.FetchBaseImageInfo(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(chainIDs(baseImage.LayerInfos)).To(ConsistOf("layer-111", "chain-222", "chain-333"))
		})

		Context("when fetching the list of layers fails", func() {
			BeforeEach(func() {
				fakeFetcher.BaseImageInfoReturns(groot.BaseImageInfo{
					LayerInfos: []groot.LayerInfo{},
					Config:     specsv1.Image{},
				}, errors.New("failed to get list of layers"))
			})

			It("returns an error", func() {
				_, err := baseImagePuller.FetchBaseImageInfo(logger)
				Expect(err).To(MatchError(ContainSubstring("failed to get list of layers")))
			})
		})
	})

	Describe("Pull", func() {
		It("creates volumes for all the layers", func() {
			err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeVolumeDriver.CreateVolumeCallCount()).To(Equal(3))
			_, parentChainID, chainID := fakeVolumeDriver.CreateVolumeArgsForCall(0)
			Expect(parentChainID).To(BeEmpty())
			Expect(chainID).To(MatchRegexp("layer-111-incomplete-\\d*-\\d*"))

			_, parentChainID, chainID = fakeVolumeDriver.CreateVolumeArgsForCall(1)
			Expect(parentChainID).To(Equal("layer-111"))
			Expect(chainID).To(MatchRegexp("chain-222-incomplete-\\d*-\\d*"))

			_, parentChainID, chainID = fakeVolumeDriver.CreateVolumeArgsForCall(2)
			Expect(parentChainID).To(Equal("chain-222"))
			Expect(chainID).To(MatchRegexp("chain-333-incomplete-\\d*-\\d*"))
		})

		It("unpacks the layers to the respective temporary volumes", func() {
			err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeUnpacker.UnpackCallCount()).To(Equal(3))
			_, unpackSpec := fakeUnpacker.UnpackArgsForCall(0)
			Expect(unpackSpec.TargetPath).To(MatchRegexp(filepath.Join(tmpVolumesDir, "layer-111-incomplete-\\d*-\\d*")))
			_, unpackSpec = fakeUnpacker.UnpackArgsForCall(1)
			Expect(unpackSpec.TargetPath).To(MatchRegexp(filepath.Join(tmpVolumesDir, "chain-222-incomplete-\\d*-\\d*")))
			_, unpackSpec = fakeUnpacker.UnpackArgsForCall(2)
			Expect(unpackSpec.TargetPath).To(MatchRegexp(filepath.Join(tmpVolumesDir, "chain-333-incomplete-\\d*-\\d*")))
		})

		Context("when there is a base directory provided on a layer", func() {
			BeforeEach(func() {
				layerInfos[1].BaseDirectory = "/home/base_directory"
			})

			It("handles base dir", func() {
				err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBaseDirHandler.HandleCallCount()).To(Equal(1))
				_, actualSpec, actualParentPath := fakeBaseDirHandler.HandleArgsForCall(0)
				Expect(actualSpec.BaseDirectory).To(Equal("/home/base_directory"))
				Expect(actualParentPath).To(Equal(filepath.Join(tmpVolumesDir, "layer-111")))
			})

			Context("when the base dir handler fails", func() {
				BeforeEach(func() {
					fakeBaseDirHandler.HandleReturns(errors.New("base-dir-handler-error"))
				})

				It("returns the error", func() {
					err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{})
					Expect(err).To(MatchError("base-dir-handler-error"))
				})
			})

			Context("when the driver fails to get the parent volume path", func() {
				BeforeEach(func() {
					fakeVolumeDriver.VolumePathReturns("", errors.New("driver-error"))
				})

				It("returns the error", func() {
					err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{})
					Expect(err).To(MatchError("driver-error"))
				})
			})
		})

		It("asks the volume driver to handle opaque whiteouts for each layer", func() {
			volumesDir, err := ioutil.TempDir("", "volumes")
			Expect(err).NotTo(HaveOccurred())

			fakeVolumeDriver.CreateVolumeStub = func(_ lager.Logger, _, id string) (string, error) {
				volumePath := filepath.Join(volumesDir, id)

				Expect(os.MkdirAll(volumePath, 0777)).To(Succeed())
				return volumePath, nil
			}

			err = baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{})

			Expect(err).NotTo(HaveOccurred())

			Expect(fakeUnpacker.UnpackCallCount()).To(Equal(3))
			_, unpackSpec := fakeUnpacker.UnpackArgsForCall(0)
			Expect(unpackSpec.TargetPath).To(MatchRegexp(filepath.Join(volumesDir, "layer-111-incomplete-\\d*-\\d*")))
			_, unpackSpec = fakeUnpacker.UnpackArgsForCall(1)
			Expect(unpackSpec.TargetPath).To(MatchRegexp(filepath.Join(volumesDir, "chain-222-incomplete-\\d*-\\d*")))
			_, unpackSpec = fakeUnpacker.UnpackArgsForCall(2)
			Expect(unpackSpec.TargetPath).To(MatchRegexp(filepath.Join(volumesDir, "chain-333-incomplete-\\d*-\\d*")))
		})

		It("unpacks the layers got from the fetcher", func() {
			fakeFetcher.StreamBlobStub = func(_ lager.Logger, layerInfo groot.LayerInfo) (io.ReadCloser, int64, error) {

				buffer := bytes.NewBuffer([]byte{})
				stream := gzip.NewWriter(buffer)
				defer stream.Close()
				_, err := stream.Write([]byte(fmt.Sprintf("layer-%s-contents", layerInfo.BlobID)))
				Expect(err).NotTo(HaveOccurred())
				return ioutil.NopCloser(buffer), 1200, nil
			}

			err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeUnpacker.UnpackCallCount()).To(Equal(3))

			validateLayer := func(idx int, expected string) {
				_, unpackSpec := fakeUnpacker.UnpackArgsForCall(idx)
				gzipReader, err := gzip.NewReader(unpackSpec.Stream)
				Expect(err).NotTo(HaveOccurred())
				contents, err := ioutil.ReadAll(gzipReader)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(expected))
			}

			validateLayer(0, "layer-i-am-a-layer-contents")
			validateLayer(1, "layer-i-am-another-layer-contents")
			validateLayer(2, "layer-i-am-the-last-layer-contents")
		})

		It("writes the metadata for each volume", func() {
			var unpackCall int
			fakeUnpacker.UnpackStub = func(_ lager.Logger, _ base_image_puller.UnpackSpec) (base_image_puller.UnpackOutput, error) {
				unpackCall++
				return base_image_puller.UnpackOutput{BytesWritten: int64(unpackCall * 100)}, nil
			}

			err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeVolumeDriver.WriteVolumeMetaCallCount()).To(Equal(3))
			_, id, metadata := fakeVolumeDriver.WriteVolumeMetaArgsForCall(0)
			Expect(id).To(Equal("layer-111"))
			Expect(metadata).To(Equal(base_image_puller.VolumeMeta{Size: 100}))

			_, id, metadata = fakeVolumeDriver.WriteVolumeMetaArgsForCall(1)
			Expect(id).To(Equal("chain-222"))
			Expect(metadata).To(Equal(base_image_puller.VolumeMeta{Size: 200}))

			_, id, metadata = fakeVolumeDriver.WriteVolumeMetaArgsForCall(2)
			Expect(id).To(Equal("chain-333"))
			Expect(metadata).To(Equal(base_image_puller.VolumeMeta{Size: 300}))
		})

		It("emits a metric with the unpack and download time for each layer", func() {
			err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(fakeMetricsEmitter.TryEmitDurationFromCallCount).Should(Equal(2 * len(layerInfos)))
		})

		It("uses the locksmith for each layer", func() {
			err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLocksmith.LockCallCount()).To(Equal(3))
			Expect(fakeLocksmith.UnlockCallCount()).To(Equal(3))

			for i, layer := range layerInfos {
				chainID := fakeLocksmith.LockArgsForCall(len(layerInfos) - 1 - i)
				Expect(chainID).To(Equal(layer.ChainID))
			}
		})

		Context("when writing volume metadata fails", func() {
			BeforeEach(func() {
				fakeVolumeDriver.WriteVolumeMetaReturns(errors.New("metadata failed"))
			})

			It("returns an error", func() {
				err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{})
				Expect(err).To(MatchError(ContainSubstring("metadata failed")))
			})
		})

		Context("when the layers size in the manifest will exceed the limit", func() {
			Context("when including the image size in the limit", func() {
				BeforeEach(func() {
					baseImageInfo = groot.BaseImageInfo{
						LayerInfos: []groot.LayerInfo{
							{Size: 1000},
							{Size: 201},
						},
					}
				})

				It("returns an error", func() {
					err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{
						DiskLimit:                 1200,
						ExcludeBaseImageFromQuota: false,
					})
					Expect(err).To(MatchError(ContainSubstring("layers exceed disk quota")))
				})

				Context("when the disk limit is zero", func() {
					It("doesn't fail", func() {
						err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{
							DiskLimit:                 0,
							ExcludeBaseImageFromQuota: false,
						})

						Expect(err).ToNot(HaveOccurred())
					})
				})
			})

			Context("when not including the image size in the limit", func() {
				It("doesn't fail", func() {
					fakeFetcher.BaseImageInfoReturns(groot.BaseImageInfo{
						LayerInfos: []groot.LayerInfo{
							{Size: 1000},
							{Size: 201},
						},
					}, nil)

					err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{
						DiskLimit:                 1024,
						ExcludeBaseImageFromQuota: true,
					})

					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Describe("volumes ownership", func() {
			var (
				spec      groot.BaseImageSpec
				volumeDir string
			)

			BeforeEach(func() {
				spec = groot.BaseImageSpec{}
				volumeDir = filepath.Join(tmpVolumesDir, "layer-111")
			})

			It("sets the ownership of the volume to the spec's owner ids", func() {
				spec.OwnerUID = 10000
				spec.OwnerGID = 5000

				err := baseImagePuller.Pull(logger, baseImageInfo, spec)
				Expect(err).NotTo(HaveOccurred())

				Expect(volumeDir).To(BeADirectory())
				stat_t, err := os.Stat(volumeDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(stat_t.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(10000)))
				Expect(stat_t.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(5000)))
			})

			Context("and both owner ids are 0", func() {
				It("doesn't enforce the ownership", func() {
					spec.OwnerUID = 0
					spec.OwnerGID = 0

					err := baseImagePuller.Pull(logger, baseImageInfo, spec)
					Expect(err).NotTo(HaveOccurred())

					Expect(volumeDir).To(BeADirectory())
					stat_t, err := os.Stat(volumeDir)
					Expect(err).NotTo(HaveOccurred())
					Expect(stat_t.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(0)))
					Expect(stat_t.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(0)))
				})
			})

			Context("and only owner uid mapping is 0", func() {
				It("enforces the ownership", func() {
					spec.OwnerUID = 0
					spec.OwnerGID = 5000

					err := baseImagePuller.Pull(logger, baseImageInfo, spec)
					Expect(err).NotTo(HaveOccurred())

					Expect(volumeDir).To(BeADirectory())
					stat_t, err := os.Stat(volumeDir)
					Expect(err).NotTo(HaveOccurred())
					Expect(stat_t.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(0)))
					Expect(stat_t.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(5000)))
				})
			})

			Context("and only owner gid mapping is 0", func() {
				It("enforces the ownership", func() {
					spec.OwnerUID = 10000
					spec.OwnerGID = 0

					err := baseImagePuller.Pull(logger, baseImageInfo, spec)
					Expect(err).NotTo(HaveOccurred())

					Expect(volumeDir).To(BeADirectory())
					stat_t, err := os.Stat(volumeDir)
					Expect(err).NotTo(HaveOccurred())
					Expect(stat_t.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(10000)))
					Expect(stat_t.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(0)))
				})
			})
		})

		Context("when all volumes exist", func() {
			BeforeEach(func() {
				fakeVolumeDriver.VolumePathReturns("/path/to/volume", nil)
			})

			It("does not try to create any layer", func() {
				err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeVolumeDriver.CreateVolumeCallCount()).To(Equal(0))
			})

			It("doesn't need to use the locksmith", func() {
				err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeLocksmith.LockCallCount()).To(Equal(0))
				Expect(fakeLocksmith.UnlockCallCount()).To(Equal(0))
			})
		})

		Context("when one volume exists", func() {
			BeforeEach(func() {
				fakeVolumeDriver.VolumePathStub = func(_ lager.Logger, id string) (string, error) {
					if id == "chain-222" {
						return "/path/to/chain-222", nil
					}
					return "", errors.New("not here")
				}
			})

			It("only creates the children of the existing volume", func() {
				err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeVolumeDriver.CreateVolumeCallCount()).To(Equal(1))
				_, _, volID := fakeVolumeDriver.CreateVolumeArgsForCall(0)
				Expect(volID).To(MatchRegexp("chain-333-incomplete-(\\d*)-(\\d*)"))
			})

			It("uses the locksmith for the other volumes", func() {
				err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeLocksmith.LockCallCount()).To(Equal(1))
				Expect(fakeLocksmith.UnlockCallCount()).To(Equal(1))

				Expect(fakeLocksmith.LockArgsForCall(0)).To(Equal("chain-333"))
			})
		})

		Context("when creating a volume fails", func() {
			BeforeEach(func() {
				fakeVolumeDriver.CreateVolumeReturns("", errors.New("failed to create volume"))
			})

			It("returns an error", func() {
				err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{})
				Expect(err).To(MatchError(ContainSubstring("failed to create volume")))
			})
		})

		Context("when streaming a blob fails", func() {
			BeforeEach(func() {
				fakeFetcher.StreamBlobReturns(nil, 0, errors.New("failed to stream blob"))
			})

			It("returns an error", func() {
				err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{})
				Expect(err).To(MatchError(ContainSubstring("failed to stream blob")))
			})
		})

		Context("when unpacking a blob fails", func() {
			BeforeEach(func() {
				count := 0
				fakeUnpacker.UnpackStub = func(_ lager.Logger, _ base_image_puller.UnpackSpec) (base_image_puller.UnpackOutput, error) {
					count++
					if count == 3 {
						return base_image_puller.UnpackOutput{}, errors.New("failed to unpack the blob")
					}

					return base_image_puller.UnpackOutput{}, nil
				}
			})

			It("returns an error", func() {
				err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{})
				Expect(err).To(MatchError(ContainSubstring("failed to unpack the blob")))
			})

			It("deletes the volume", func() {
				err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{})
				Expect(err).To(MatchError(ContainSubstring("failed to unpack the blob")))

				Expect(fakeVolumeDriver.DestroyVolumeCallCount()).To(Equal(1))
				_, path := fakeVolumeDriver.DestroyVolumeArgsForCall(0)
				Expect(path).To(Equal("chain-333"))
			})

			It("emits a metric with the unpack and download time for each layer", func() {
				downloadTimeMetrics := 0
				unpackTimeMetrics := 0
				mutex := &sync.Mutex{}

				fakeMetricsEmitter.TryEmitDurationFromStub = func(_ lager.Logger, name string, value time.Time) {
					mutex.Lock()
					defer mutex.Unlock()

					switch name {
					case base_image_puller.MetricsUnpackTimeName:
						unpackTimeMetrics += 1
					case base_image_puller.MetricsDownloadTimeName:
						downloadTimeMetrics += 1
					}
				}

				err := baseImagePuller.Pull(logger, baseImageInfo, groot.BaseImageSpec{})
				Expect(err).To(MatchError(ContainSubstring("failed to unpack the blob")))

				Eventually(func() int {
					mutex.Lock()
					defer mutex.Unlock()
					return unpackTimeMetrics
				}).Should(Equal(3), "incorrect number of unpack time metrics emitted")
				Eventually(func() int {
					mutex.Lock()
					defer mutex.Unlock()
					return downloadTimeMetrics
				}).Should(Equal(3), "incorrect number of download time metrics emitted")
			})

			Context("when UID and GID mappings are provided", func() {
				var spec groot.BaseImageSpec

				BeforeEach(func() {
					spec = groot.BaseImageSpec{
						UIDMappings: []groot.IDMappingSpec{
							{
								HostID:      1,
								NamespaceID: 1,
								Size:        1,
							},
						},
						GIDMappings: []groot.IDMappingSpec{
							{
								HostID:      100,
								NamespaceID: 100,
								Size:        100,
							},
						},
					}
				})

				It("deletes the namespaced volume", func() {
					err := baseImagePuller.Pull(logger, baseImageInfo, spec)
					Expect(err).To(HaveOccurred())

					Expect(fakeVolumeDriver.DestroyVolumeCallCount()).To(Equal(1))
					_, path := fakeVolumeDriver.DestroyVolumeArgsForCall(0)
					Expect(path).To(Equal("chain-333"))
				})
			})
		})
	})
})

func chainIDs(layerInfos []groot.LayerInfo) []string {
	chainIDs := []string{}
	for _, layerInfo := range layerInfos {
		chainIDs = append(chainIDs, layerInfo.ChainID)
	}
	return chainIDs
}

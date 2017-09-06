package base_image_puller_test

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("Base Image Puller", func() {
	var (
		logger                   lager.Logger
		fakeTarFetcher           *base_image_pullerfakes.FakeFetcher
		fakeLayerFetcher         *base_image_pullerfakes.FakeFetcher
		fakeUnpacker             *base_image_pullerfakes.FakeUnpacker
		fakeVolumeDriver         *base_image_pullerfakes.FakeVolumeDriver
		fakeLocksmith            *grootfakes.FakeLocksmith
		fakeMetricsEmitter       *grootfakes.FakeMetricsEmitter
		fakeDependencyRegisterer *base_image_pullerfakes.FakeDependencyRegisterer
		expectedImgDesc          specsv1.Image

		baseImagePuller *base_image_puller.BaseImagePuller
		layersDigest    []base_image_puller.LayerDigest

		baseImageSrcURL *url.URL
		fakeVolumePath  string
	)

	BeforeEach(func() {
		fakeUnpacker = new(base_image_pullerfakes.FakeUnpacker)

		fakeLocksmith = new(grootfakes.FakeLocksmith)
		fakeMetricsEmitter = new(grootfakes.FakeMetricsEmitter)
		fakeTarFetcher = new(base_image_pullerfakes.FakeFetcher)
		fakeLayerFetcher = new(base_image_pullerfakes.FakeFetcher)
		expectedImgDesc = specsv1.Image{Author: "Groot"}
		layersDigest = []base_image_puller.LayerDigest{
			{BlobID: "i-am-a-layer", ChainID: "layer-111", ParentChainID: ""},
			{BlobID: "i-am-another-layer", ChainID: "chain-222", ParentChainID: "layer-111"},
			{BlobID: "i-am-the-last-layer", ChainID: "chain-333", ParentChainID: "chain-222"},
		}
		fakeLayerFetcher.BaseImageInfoReturns(
			base_image_puller.BaseImageInfo{
				LayersDigest: layersDigest,
				Config:       expectedImgDesc,
			}, nil)

		fakeLayerFetcher.StreamBlobStub = func(_ lager.Logger, baseImageURL *url.URL, source string) (io.ReadCloser, int64, error) {
			buffer := bytes.NewBuffer([]byte{})
			stream := gzip.NewWriter(buffer)
			defer stream.Close()
			return ioutil.NopCloser(buffer), 0, nil
		}

		fakeVolumeDriver = new(base_image_pullerfakes.FakeVolumeDriver)
		fakeVolumeDriver.VolumePathReturns("", errors.New("volume does not exist"))
		fakeVolumeDriver.CreateVolumeStub = func(_ lager.Logger, _, _ string) (string, error) {
			var err error
			fakeVolumePath, err = ioutil.TempDir("", "volume")
			return fakeVolumePath, err
		}

		fakeDependencyRegisterer = new(base_image_pullerfakes.FakeDependencyRegisterer)

		baseImagePuller = base_image_puller.NewBaseImagePuller(fakeTarFetcher, fakeLayerFetcher, fakeUnpacker, fakeVolumeDriver, fakeDependencyRegisterer, fakeMetricsEmitter, fakeLocksmith)
		logger = lagertest.NewTestLogger("image-puller")

		var err error
		baseImageSrcURL, err = url.Parse("docker:///an/image")
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns the image description", func() {
		baseImage, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
			BaseImageSrc: baseImageSrcURL,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(baseImage.BaseImage).To(Equal(expectedImgDesc))
	})

	It("returns the chain ids", func() {
		baseImage, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
			BaseImageSrc: baseImageSrcURL,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(baseImage.ChainIDs).To(ConsistOf("layer-111", "chain-222", "chain-333"))
	})

	It("creates volumes for all the layers", func() {
		_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
			BaseImageSrc: baseImageSrcURL,
		})
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
		volumesDir, err := ioutil.TempDir("", "volumes")
		Expect(err).NotTo(HaveOccurred())

		fakeVolumeDriver.CreateVolumeStub = func(_ lager.Logger, _, id string) (string, error) {
			volumePath := filepath.Join(volumesDir, id)

			Expect(os.MkdirAll(volumePath, 0777)).To(Succeed())
			return volumePath, nil
		}

		_, err = baseImagePuller.Pull(logger, groot.BaseImageSpec{
			BaseImageSrc: baseImageSrcURL,
		})

		Expect(err).NotTo(HaveOccurred())

		Expect(fakeUnpacker.UnpackCallCount()).To(Equal(3))
		_, unpackSpec := fakeUnpacker.UnpackArgsForCall(0)
		Expect(unpackSpec.TargetPath).To(MatchRegexp(filepath.Join(volumesDir, "layer-111-incomplete-\\d*-\\d*")))
		_, unpackSpec = fakeUnpacker.UnpackArgsForCall(1)
		Expect(unpackSpec.TargetPath).To(MatchRegexp(filepath.Join(volumesDir, "chain-222-incomplete-\\d*-\\d*")))
		_, unpackSpec = fakeUnpacker.UnpackArgsForCall(2)
		Expect(unpackSpec.TargetPath).To(MatchRegexp(filepath.Join(volumesDir, "chain-333-incomplete-\\d*-\\d*")))
	})

	It("asks the volume driver to handle opaque whiteouts for each layer", func() {
		volumesDir, err := ioutil.TempDir("", "volumes")
		Expect(err).NotTo(HaveOccurred())

		fakeVolumeDriver.CreateVolumeStub = func(_ lager.Logger, _, id string) (string, error) {
			volumePath := filepath.Join(volumesDir, id)

			Expect(os.MkdirAll(volumePath, 0777)).To(Succeed())
			return volumePath, nil
		}

		_, err = baseImagePuller.Pull(logger, groot.BaseImageSpec{
			BaseImageSrc: baseImageSrcURL,
		})

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
		fakeLayerFetcher.StreamBlobStub = func(_ lager.Logger, baseImageURL *url.URL, source string) (io.ReadCloser, int64, error) {
			Expect(baseImageURL).To(Equal(baseImageSrcURL))

			buffer := bytes.NewBuffer([]byte{})
			stream := gzip.NewWriter(buffer)
			defer stream.Close()
			_, err := stream.Write([]byte(fmt.Sprintf("layer-%s-contents", source)))
			Expect(err).NotTo(HaveOccurred())
			return ioutil.NopCloser(buffer), 1200, nil
		}

		_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
			BaseImageSrc: baseImageSrcURL,
		})
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

	It("registers chain ids used by a image", func() {
		_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
			BaseImageSrc: baseImageSrcURL,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeDependencyRegisterer.RegisterCallCount()).To(Equal(1))
		imageID, chainIDs := fakeDependencyRegisterer.RegisterArgsForCall(0)
		Expect(imageID).To(Equal("baseimage:docker:///an/image"))
		Expect(chainIDs).To(ConsistOf("layer-111", "chain-222", "chain-333"))
	})

	It("writes the metadata for each volume", func() {
		var unpackCall int
		fakeUnpacker.UnpackStub = func(_ lager.Logger, _ base_image_puller.UnpackSpec) (base_image_puller.UnpackOutput, error) {
			unpackCall++
			return base_image_puller.UnpackOutput{BytesWritten: int64(unpackCall * 100)}, nil
		}

		_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
			BaseImageSrc: baseImageSrcURL,
		})
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
		_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
			BaseImageSrc: baseImageSrcURL,
		})
		Expect(err).NotTo(HaveOccurred())

		Eventually(fakeMetricsEmitter.TryEmitDurationFromCallCount).Should(Equal(2 * len(layersDigest)))
	})

	It("uses the locksmith for each layer", func() {
		_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
			BaseImageSrc: baseImageSrcURL,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeLocksmith.LockCallCount()).To(Equal(3))
		Expect(fakeLocksmith.UnlockCallCount()).To(Equal(3))

		for i, layer := range layersDigest {
			chainID := fakeLocksmith.LockArgsForCall(len(layersDigest) - 1 - i)
			Expect(chainID).To(Equal(layer.ChainID))
		}
	})

	Context("when writing volume metadata fails", func() {
		BeforeEach(func() {
			fakeVolumeDriver.WriteVolumeMetaReturns(errors.New("metadata failed"))
		})

		It("returns an error", func() {
			_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
				BaseImageSrc: baseImageSrcURL,
			})
			Expect(err).To(MatchError(ContainSubstring("metadata failed")))
		})
	})

	Context("when registration fails", func() {
		It("returns an error", func() {
			fakeDependencyRegisterer.RegisterReturns(
				errors.New("failed to register base image dependencies"),
			)

			_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
				BaseImageSrc: baseImageSrcURL,
			})
			Expect(err).To(MatchError(ContainSubstring("failed to register base image dependencies")))
		})
	})

	Context("deciding between tar and layer fetcher", func() {
		It("uses tar fetcher when the image url doesn't have a schema", func() {
			imageSrc, err := url.Parse("/path/to/my/image")
			Expect(err).NotTo(HaveOccurred())

			_, err = baseImagePuller.Pull(logger, groot.BaseImageSpec{
				BaseImageSrc: imageSrc,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeTarFetcher.BaseImageInfoCallCount()).To(Equal(1))
			Expect(fakeLayerFetcher.BaseImageInfoCallCount()).To(Equal(0))
		})

		It("uses remote fetcher when the image url does have a schema", func() {
			imageSrc, err := url.Parse("crazy://image/place")
			Expect(err).NotTo(HaveOccurred())

			_, err = baseImagePuller.Pull(logger, groot.BaseImageSpec{
				BaseImageSrc: imageSrc,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeTarFetcher.BaseImageInfoCallCount()).To(Equal(0))
			Expect(fakeLayerFetcher.BaseImageInfoCallCount()).To(Equal(1))
		})
	})

	Context("when the layers size in the manifest will exceed the limit", func() {
		Context("when including the image size in the limit", func() {
			It("returns an error", func() {
				fakeLayerFetcher.BaseImageInfoReturns(base_image_puller.BaseImageInfo{
					LayersDigest: []base_image_puller.LayerDigest{
						{Size: 1000},
						{Size: 201},
					},
				}, nil)

				_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
					BaseImageSrc:              baseImageSrcURL,
					DiskLimit:                 1200,
					ExcludeBaseImageFromQuota: false,
				})
				Expect(err).To(MatchError(ContainSubstring("layers exceed disk quota")))
			})

			Context("when the disk limit is zero", func() {
				It("doesn't fail", func() {
					fakeLayerFetcher.BaseImageInfoReturns(base_image_puller.BaseImageInfo{
						LayersDigest: []base_image_puller.LayerDigest{
							{Size: 1000},
							{Size: 201},
						},
					}, nil)

					_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
						BaseImageSrc:              baseImageSrcURL,
						DiskLimit:                 0,
						ExcludeBaseImageFromQuota: false,
					})

					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when not including the image size in the limit", func() {
			It("doesn't fail", func() {
				fakeLayerFetcher.BaseImageInfoReturns(base_image_puller.BaseImageInfo{
					LayersDigest: []base_image_puller.LayerDigest{
						{Size: 1000},
						{Size: 201},
					},
				}, nil)

				_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
					BaseImageSrc:              baseImageSrcURL,
					DiskLimit:                 1024,
					ExcludeBaseImageFromQuota: true,
				})

				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Context("when fetching the list of layers fails", func() {
		BeforeEach(func() {
			fakeLayerFetcher.BaseImageInfoReturns(base_image_puller.BaseImageInfo{
				LayersDigest: []base_image_puller.LayerDigest{},
				Config:       specsv1.Image{},
			}, errors.New("failed to get list of layers"))
		})

		It("returns an error", func() {
			_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
				BaseImageSrc: baseImageSrcURL,
			})
			Expect(err).To(MatchError(ContainSubstring("failed to get list of layers")))
		})
	})

	Context("when UID and GID mappings are provided", func() {
		var spec groot.BaseImageSpec

		BeforeEach(func() {
			spec = groot.BaseImageSpec{
				BaseImageSrc: baseImageSrcURL,
				UIDMappings: []groot.IDMappingSpec{
					{
						HostID:      os.Getuid(),
						NamespaceID: 0,
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

		It("applies the UID and GID mappings in the unpacked blobs", func() {
			_, err := baseImagePuller.Pull(logger, spec)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeUnpacker.UnpackCallCount()).To(Equal(3))
			_, unpackSpec := fakeUnpacker.UnpackArgsForCall(0)
			Expect(unpackSpec.UIDMappings).To(Equal(spec.UIDMappings))
			Expect(unpackSpec.GIDMappings).To(Equal(spec.GIDMappings))

			_, unpackSpec = fakeUnpacker.UnpackArgsForCall(1)
			Expect(unpackSpec.UIDMappings).To(Equal(spec.UIDMappings))
			Expect(unpackSpec.GIDMappings).To(Equal(spec.GIDMappings))

			_, unpackSpec = fakeUnpacker.UnpackArgsForCall(2)
			Expect(unpackSpec.UIDMappings).To(Equal(spec.UIDMappings))
			Expect(unpackSpec.GIDMappings).To(Equal(spec.GIDMappings))
		})
	})

	Describe("volumes ownership", func() {
		var spec groot.BaseImageSpec

		BeforeEach(func() {
			spec = groot.BaseImageSpec{
				BaseImageSrc: baseImageSrcURL,
			}
		})

		It("sets the ownership of the store to the spec's owner ids", func() {
			spec.OwnerUID = 10000
			spec.OwnerGID = 5000

			_, err := baseImagePuller.Pull(logger, spec)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeVolumePath).To(BeADirectory())
			volumePath, err := os.Stat(fakeVolumePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(volumePath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(10000)))
			Expect(volumePath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(5000)))
		})

		Context("and both owner ids are 0", func() {
			It("doesn't enforce the ownership", func() {
				spec.OwnerUID = 0
				spec.OwnerGID = 0

				_, err := baseImagePuller.Pull(logger, spec)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeVolumePath).To(BeADirectory())
				volumePath, err := os.Stat(fakeVolumePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(volumePath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(0)))
				Expect(volumePath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(0)))
			})
		})

		Context("and only owner uid mapping is 0", func() {
			It("enforces the ownership", func() {
				spec.OwnerUID = 0
				spec.OwnerGID = 5000

				_, err := baseImagePuller.Pull(logger, spec)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeVolumePath).To(BeADirectory())
				volumePath, err := os.Stat(fakeVolumePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(volumePath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(0)))
				Expect(volumePath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(5000)))
			})
		})

		Context("and only owner gid mapping is 0", func() {
			It("enforces the ownership", func() {
				spec.OwnerUID = 10000
				spec.OwnerGID = 0

				_, err := baseImagePuller.Pull(logger, spec)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeVolumePath).To(BeADirectory())
				volumePath, err := os.Stat(fakeVolumePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(volumePath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(10000)))
				Expect(volumePath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(0)))
			})
		})
	})

	Context("when all volumes exist", func() {
		BeforeEach(func() {
			fakeVolumeDriver.VolumePathReturns("/path/to/volume", nil)
		})

		It("does not try to create any layer", func() {
			_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
				BaseImageSrc: baseImageSrcURL,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeVolumeDriver.CreateVolumeCallCount()).To(Equal(0))
		})

		It("doesn't need to use the locksmith", func() {
			_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
				BaseImageSrc: baseImageSrcURL,
			})
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
			_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
				BaseImageSrc: baseImageSrcURL,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeVolumeDriver.CreateVolumeCallCount()).To(Equal(1))
			_, _, volID := fakeVolumeDriver.CreateVolumeArgsForCall(0)
			Expect(volID).To(MatchRegexp("chain-333-incomplete-(\\d*)-(\\d*)"))
		})

		It("uses the locksmith for the other volumes", func() {
			_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
				BaseImageSrc: baseImageSrcURL,
			})
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
			_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
				BaseImageSrc: baseImageSrcURL,
			})
			Expect(err).To(MatchError(ContainSubstring("failed to create volume")))
		})
	})

	Context("when streaming a blob fails", func() {
		BeforeEach(func() {
			fakeLayerFetcher.StreamBlobReturns(nil, 0, errors.New("failed to stream blob"))
		})

		It("returns an error", func() {
			_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{BaseImageSrc: baseImageSrcURL})
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
			_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{BaseImageSrc: baseImageSrcURL})
			Expect(err).To(MatchError(ContainSubstring("failed to unpack the blob")))
		})

		It("deletes the volume", func() {
			_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{BaseImageSrc: baseImageSrcURL})
			Expect(err).To(HaveOccurred())

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

			_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
				BaseImageSrc: baseImageSrcURL,
			})
			Expect(err).To(HaveOccurred())

			Eventually(fakeMetricsEmitter.TryEmitDurationFromCallCount).Should(Equal(6))
			Eventually(func() int {
				mutex.Lock()
				defer mutex.Unlock()
				return unpackTimeMetrics
			}).Should(Equal(3))
			Eventually(func() int {
				mutex.Lock()
				defer mutex.Unlock()
				return downloadTimeMetrics
			}).Should(Equal(3))
		})

		Context("when UID and GID mappings are provided", func() {
			var spec groot.BaseImageSpec

			BeforeEach(func() {
				spec = groot.BaseImageSpec{
					BaseImageSrc: baseImageSrcURL,
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
				_, err := baseImagePuller.Pull(logger, spec)
				Expect(err).To(HaveOccurred())

				Expect(fakeVolumeDriver.DestroyVolumeCallCount()).To(Equal(1))
				_, path := fakeVolumeDriver.DestroyVolumeArgsForCall(0)
				Expect(path).To(Equal("chain-333"))
			})
		})
	})
})

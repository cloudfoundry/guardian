package base_image_puller_test

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"

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
		logger              lager.Logger
		fakeLocalFetcher    *base_image_pullerfakes.FakeFetcher
		fakeRemoteFetcher   *base_image_pullerfakes.FakeFetcher
		fakeBaseImagePuller *grootfakes.FakeBaseImagePuller
		fakeUnpacker        *base_image_pullerfakes.FakeUnpacker
		fakeVolumeDriver    *base_image_pullerfakes.FakeVolumeDriver
		expectedImgDesc     specsv1.Image

		baseImagePuller *base_image_puller.BaseImagePuller

		remoteBaseImageSrc *url.URL
	)

	BeforeEach(func() {
		fakeBaseImagePuller = new(grootfakes.FakeBaseImagePuller)

		fakeUnpacker = new(base_image_pullerfakes.FakeUnpacker)

		fakeLocalFetcher = new(base_image_pullerfakes.FakeFetcher)
		fakeRemoteFetcher = new(base_image_pullerfakes.FakeFetcher)
		expectedImgDesc = specsv1.Image{Author: "Groot"}
		fakeRemoteFetcher.BaseImageInfoReturns(
			base_image_puller.BaseImageInfo{
				LayersDigest: []base_image_puller.LayerDigest{
					base_image_puller.LayerDigest{BlobID: "i-am-a-layer", ChainID: "layer-111", ParentChainID: ""},
					base_image_puller.LayerDigest{BlobID: "i-am-another-layer", ChainID: "chain-222", ParentChainID: "layer-111"},
					base_image_puller.LayerDigest{BlobID: "i-am-the-last-layer", ChainID: "chain-333", ParentChainID: "chain-222"},
				},
				Config: expectedImgDesc,
			}, nil)

		fakeRemoteFetcher.StreamBlobStub = func(_ lager.Logger, baseImageURL *url.URL, source string) (io.ReadCloser, int64, error) {
			buffer := bytes.NewBuffer([]byte{})
			stream := gzip.NewWriter(buffer)
			defer stream.Close()
			return ioutil.NopCloser(buffer), 0, nil
		}

		fakeVolumeDriver = new(base_image_pullerfakes.FakeVolumeDriver)
		fakeVolumeDriver.PathReturns("", errors.New("volume does not exist"))

		baseImagePuller = base_image_puller.NewBaseImagePuller(fakeLocalFetcher, fakeRemoteFetcher, fakeUnpacker, fakeVolumeDriver)
		logger = lagertest.NewTestLogger("image-puller")

		var err error
		remoteBaseImageSrc, err = url.Parse("docker:///an/image")
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns the image description", func() {
		baseImage, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
			BaseImageSrc: remoteBaseImageSrc,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(baseImage.BaseImage).To(Equal(expectedImgDesc))
	})

	It("returns the last volume's path", func() {
		fakeVolumeDriver.PathStub = func(_ lager.Logger, id string) (string, error) {
			return fmt.Sprintf("/path/to/volume/%s", id), nil
		}

		baseImage, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
			BaseImageSrc: remoteBaseImageSrc,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(baseImage.VolumePath).To(Equal("/path/to/volume/chain-333"))
	})

	It("returns the chain ids", func() {
		baseImage, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
			BaseImageSrc: remoteBaseImageSrc,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(baseImage.ChainIDs).To(ConsistOf("layer-111", "chain-222", "chain-333"))
	})

	It("creates volumes for all the layers", func() {
		_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
			BaseImageSrc: remoteBaseImageSrc,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeVolumeDriver.CreateCallCount()).To(Equal(3))
		_, parentChainID, chainID := fakeVolumeDriver.CreateArgsForCall(0)
		Expect(parentChainID).To(BeEmpty())
		Expect(chainID).To(Equal("layer-111"))

		_, parentChainID, chainID = fakeVolumeDriver.CreateArgsForCall(1)
		Expect(parentChainID).To(Equal("layer-111"))
		Expect(chainID).To(Equal("chain-222"))

		_, parentChainID, chainID = fakeVolumeDriver.CreateArgsForCall(2)
		Expect(parentChainID).To(Equal("chain-222"))
		Expect(chainID).To(Equal("chain-333"))
	})

	It("unpacks the layers to the respective volumes", func() {
		fakeVolumeDriver.CreateStub = func(_ lager.Logger, _, id string) (string, error) {
			return fmt.Sprintf("/volume/%s", id), nil
		}

		_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
			BaseImageSrc: remoteBaseImageSrc,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeUnpacker.UnpackCallCount()).To(Equal(3))
		_, unpackSpec := fakeUnpacker.UnpackArgsForCall(0)
		Expect(unpackSpec.TargetPath).To(Equal("/volume/layer-111"))
		_, unpackSpec = fakeUnpacker.UnpackArgsForCall(1)
		Expect(unpackSpec.TargetPath).To(Equal("/volume/chain-222"))
		_, unpackSpec = fakeUnpacker.UnpackArgsForCall(2)
		Expect(unpackSpec.TargetPath).To(Equal("/volume/chain-333"))
	})

	It("unpacks the layers got from the fetcher", func() {
		fakeRemoteFetcher.StreamBlobStub = func(_ lager.Logger, baseImageURL *url.URL, source string) (io.ReadCloser, int64, error) {
			Expect(baseImageURL).To(Equal(remoteBaseImageSrc))

			buffer := bytes.NewBuffer([]byte{})
			stream := gzip.NewWriter(buffer)
			defer stream.Close()
			stream.Write([]byte(fmt.Sprintf("layer-%s-contents", source)))
			return ioutil.NopCloser(buffer), 1200, nil
		}

		_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
			BaseImageSrc: remoteBaseImageSrc,
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

	Context("deciding between local and remote fetcher", func() {
		It("uses local fetcher when the image url doesn't have a schema", func() {
			imageSrc, err := url.Parse("/path/to/my/image")
			Expect(err).NotTo(HaveOccurred())

			_, err = baseImagePuller.Pull(logger, groot.BaseImageSpec{
				BaseImageSrc: imageSrc,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLocalFetcher.BaseImageInfoCallCount()).To(Equal(1))
			Expect(fakeRemoteFetcher.BaseImageInfoCallCount()).To(Equal(0))
		})

		It("uses remote fetcher when the image url does have a schema", func() {
			imageSrc, err := url.Parse("crazy://image/place")
			Expect(err).NotTo(HaveOccurred())

			_, err = baseImagePuller.Pull(logger, groot.BaseImageSpec{
				BaseImageSrc: imageSrc,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLocalFetcher.BaseImageInfoCallCount()).To(Equal(0))
			Expect(fakeRemoteFetcher.BaseImageInfoCallCount()).To(Equal(1))
		})
	})

	Context("when the layers size in the manifest will exceed the limit", func() {
		Context("when including the image size in the limit", func() {
			It("returns an error", func() {
				fakeRemoteFetcher.BaseImageInfoReturns(base_image_puller.BaseImageInfo{
					LayersDigest: []base_image_puller.LayerDigest{
						base_image_puller.LayerDigest{Size: 1000},
						base_image_puller.LayerDigest{Size: 201},
					},
				}, nil)

				_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
					BaseImageSrc:              remoteBaseImageSrc,
					DiskLimit:                 1200,
					ExcludeBaseImageFromQuota: false,
				})
				Expect(err).To(MatchError(ContainSubstring("layers exceed disk quota")))
			})

			Context("when the disk limit is zero", func() {
				It("doesn't fail", func() {
					fakeRemoteFetcher.BaseImageInfoReturns(base_image_puller.BaseImageInfo{
						LayersDigest: []base_image_puller.LayerDigest{
							base_image_puller.LayerDigest{Size: 1000},
							base_image_puller.LayerDigest{Size: 201},
						},
					}, nil)

					_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
						BaseImageSrc:              remoteBaseImageSrc,
						DiskLimit:                 0,
						ExcludeBaseImageFromQuota: false,
					})

					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when not including the image size in the limit", func() {
			It("doesn't fail", func() {
				fakeRemoteFetcher.BaseImageInfoReturns(base_image_puller.BaseImageInfo{
					LayersDigest: []base_image_puller.LayerDigest{
						base_image_puller.LayerDigest{Size: 1000},
						base_image_puller.LayerDigest{Size: 201},
					},
				}, nil)

				_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
					BaseImageSrc:              remoteBaseImageSrc,
					DiskLimit:                 1024,
					ExcludeBaseImageFromQuota: true,
				})

				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Context("when fetching the list of layers fails", func() {
		BeforeEach(func() {
			fakeRemoteFetcher.BaseImageInfoReturns(base_image_puller.BaseImageInfo{
				LayersDigest: []base_image_puller.LayerDigest{},
				Config:       specsv1.Image{},
			}, errors.New("failed to get list of layers"))
		})

		It("returns an error", func() {
			_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
				BaseImageSrc: remoteBaseImageSrc,
			})
			Expect(err).To(MatchError(ContainSubstring("failed to get list of layers")))
		})
	})

	Context("when UID and GID mappings are provided", func() {
		var spec groot.BaseImageSpec

		BeforeEach(func() {
			spec = groot.BaseImageSpec{
				BaseImageSrc: remoteBaseImageSrc,
				UIDMappings: []groot.IDMappingSpec{
					groot.IDMappingSpec{
						HostID:      1,
						NamespaceID: 1,
						Size:        1,
					},
				},
				GIDMappings: []groot.IDMappingSpec{
					groot.IDMappingSpec{
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

	Context("when all volumes exist", func() {
		BeforeEach(func() {
			fakeVolumeDriver.PathReturns("/path/to/volume", nil)
		})

		It("does not try to create any layer", func() {
			_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
				BaseImageSrc: remoteBaseImageSrc,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeVolumeDriver.CreateCallCount()).To(Equal(0))
		})
	})

	Context("when one volume exists", func() {
		BeforeEach(func() {
			fakeVolumeDriver.PathStub = func(_ lager.Logger, id string) (string, error) {
				if id == "chain-222" {
					return "/path/to/chain-222", nil
				}
				return "", errors.New("not here")
			}
		})

		It("only creates the children of the existing volume", func() {
			_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
				BaseImageSrc: remoteBaseImageSrc,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeVolumeDriver.CreateCallCount()).To(Equal(1))
			_, _, volID := fakeVolumeDriver.CreateArgsForCall(0)
			Expect(volID).To(Equal("chain-333"))
		})
	})

	Context("when creating a volume fails", func() {
		BeforeEach(func() {
			fakeVolumeDriver.CreateReturns("", errors.New("failed to create volume"))
		})

		It("returns an error", func() {
			_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{
				BaseImageSrc: remoteBaseImageSrc,
			})
			Expect(err).To(MatchError(ContainSubstring("failed to create volume")))
		})
	})

	Context("when streaming a blob fails", func() {
		BeforeEach(func() {
			fakeRemoteFetcher.StreamBlobReturns(nil, 0, errors.New("failed to stream blob"))
		})

		It("returns an error", func() {
			_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{BaseImageSrc: remoteBaseImageSrc})
			Expect(err).To(MatchError(ContainSubstring("failed to stream blob")))
		})
	})

	Context("when unpacking a blob fails", func() {
		BeforeEach(func() {
			count := 0
			fakeUnpacker.UnpackStub = func(_ lager.Logger, _ base_image_puller.UnpackSpec) error {
				count++
				if count == 3 {
					return errors.New("failed to unpack the blob")
				}

				return nil
			}
		})

		It("returns an error", func() {
			_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{BaseImageSrc: remoteBaseImageSrc})
			Expect(err).To(MatchError(ContainSubstring("failed to unpack the blob")))
		})

		It("deletes the volume", func() {
			_, err := baseImagePuller.Pull(logger, groot.BaseImageSpec{BaseImageSrc: remoteBaseImageSrc})
			Expect(err).To(HaveOccurred())

			Expect(fakeVolumeDriver.DestroyVolumeCallCount()).To(Equal(1))
			_, path := fakeVolumeDriver.DestroyVolumeArgsForCall(0)
			Expect(path).To(Equal("chain-333"))
		})

		Context("when UID and GID mappings are provided", func() {
			var spec groot.BaseImageSpec

			BeforeEach(func() {
				spec = groot.BaseImageSpec{
					BaseImageSrc: remoteBaseImageSrc,
					UIDMappings: []groot.IDMappingSpec{
						groot.IDMappingSpec{
							HostID:      1,
							NamespaceID: 1,
							Size:        1,
						},
					},
					GIDMappings: []groot.IDMappingSpec{
						groot.IDMappingSpec{
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

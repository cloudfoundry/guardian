package image_puller_test

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/groot/grootfakes"
	"code.cloudfoundry.org/grootfs/image_puller"
	"code.cloudfoundry.org/grootfs/image_puller/image_pullerfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("Image Puller", func() {
	var (
		logger            lager.Logger
		fakeLocalFetcher  *image_pullerfakes.FakeFetcher
		fakeRemoteFetcher *image_pullerfakes.FakeFetcher
		fakeImagePuller   *grootfakes.FakeImagePuller
		fakeUnpacker      *image_pullerfakes.FakeUnpacker
		fakeVolumeDriver  *image_pullerfakes.FakeVolumeDriver
		expectedImgDesc   specsv1.Image

		imagePuller *image_puller.ImagePuller

		remoteImageSrc *url.URL
	)

	BeforeEach(func() {
		fakeImagePuller = new(grootfakes.FakeImagePuller)

		fakeUnpacker = new(image_pullerfakes.FakeUnpacker)

		fakeLocalFetcher = new(image_pullerfakes.FakeFetcher)
		fakeRemoteFetcher = new(image_pullerfakes.FakeFetcher)
		expectedImgDesc = specsv1.Image{Author: "Groot"}
		fakeRemoteFetcher.ImageInfoReturns(
			image_puller.ImageInfo{
				LayersDigest: []image_puller.LayerDigest{
					image_puller.LayerDigest{BlobID: "i-am-a-layer", ChainID: "layer-111", ParentChainID: ""},
					image_puller.LayerDigest{BlobID: "i-am-another-layer", ChainID: "chain-222", ParentChainID: "layer-111"},
					image_puller.LayerDigest{BlobID: "i-am-the-last-layer", ChainID: "chain-333", ParentChainID: "chain-222"},
				},
				Config: expectedImgDesc,
			}, nil)

		fakeRemoteFetcher.StreamBlobStub = func(_ lager.Logger, imageURL *url.URL, source string) (io.ReadCloser, int64, error) {
			buffer := bytes.NewBuffer([]byte{})
			stream := gzip.NewWriter(buffer)
			defer stream.Close()
			return ioutil.NopCloser(buffer), 0, nil
		}

		fakeVolumeDriver = new(image_pullerfakes.FakeVolumeDriver)
		fakeVolumeDriver.PathReturns("", errors.New("volume does not exist"))

		imagePuller = image_puller.NewImagePuller(fakeLocalFetcher, fakeRemoteFetcher, fakeUnpacker, fakeVolumeDriver)
		logger = lagertest.NewTestLogger("image-puller")

		var err error
		remoteImageSrc, err = url.Parse("docker:///an/image")
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns the image description", func() {
		image, err := imagePuller.Pull(logger, groot.ImageSpec{
			ImageSrc: remoteImageSrc,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(image.Image).To(Equal(expectedImgDesc))
	})

	It("returns the last volume's path", func() {
		fakeVolumeDriver.PathStub = func(_ lager.Logger, id string) (string, error) {
			return fmt.Sprintf("/path/to/volume/%s", id), nil
		}

		image, err := imagePuller.Pull(logger, groot.ImageSpec{
			ImageSrc: remoteImageSrc,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(image.VolumePath).To(Equal("/path/to/volume/chain-333"))
	})

	It("returns the chain ids", func() {
		image, err := imagePuller.Pull(logger, groot.ImageSpec{
			ImageSrc: remoteImageSrc,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(image.ChainIDs).To(ConsistOf("layer-111", "chain-222", "chain-333"))
	})

	It("creates volumes for all the layers", func() {
		_, err := imagePuller.Pull(logger, groot.ImageSpec{
			ImageSrc: remoteImageSrc,
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

		_, err := imagePuller.Pull(logger, groot.ImageSpec{
			ImageSrc: remoteImageSrc,
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
		fakeRemoteFetcher.StreamBlobStub = func(_ lager.Logger, imageURL *url.URL, source string) (io.ReadCloser, int64, error) {
			Expect(imageURL).To(Equal(remoteImageSrc))

			buffer := bytes.NewBuffer([]byte{})
			stream := gzip.NewWriter(buffer)
			defer stream.Close()
			stream.Write([]byte(fmt.Sprintf("layer-%s-contents", source)))
			return ioutil.NopCloser(buffer), 1200, nil
		}

		_, err := imagePuller.Pull(logger, groot.ImageSpec{
			ImageSrc: remoteImageSrc,
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

			_, err = imagePuller.Pull(logger, groot.ImageSpec{
				ImageSrc: imageSrc,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLocalFetcher.ImageInfoCallCount()).To(Equal(1))
			Expect(fakeRemoteFetcher.ImageInfoCallCount()).To(Equal(0))
		})

		It("uses remote fetcher when the image url does have a schema", func() {
			imageSrc, err := url.Parse("crazy://image/place")
			Expect(err).NotTo(HaveOccurred())

			_, err = imagePuller.Pull(logger, groot.ImageSpec{
				ImageSrc: imageSrc,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLocalFetcher.ImageInfoCallCount()).To(Equal(0))
			Expect(fakeRemoteFetcher.ImageInfoCallCount()).To(Equal(1))
		})
	})

	Context("when the layers size in the manifest will exceed the limit", func() {
		Context("when including the image size in the limit", func() {
			It("returns an error", func() {
				fakeRemoteFetcher.ImageInfoReturns(image_puller.ImageInfo{
					LayersDigest: []image_puller.LayerDigest{
						image_puller.LayerDigest{Size: 1000},
						image_puller.LayerDigest{Size: 201},
					},
				}, nil)

				_, err := imagePuller.Pull(logger, groot.ImageSpec{
					ImageSrc:              remoteImageSrc,
					DiskLimit:             1200,
					ExcludeImageFromQuota: false,
				})
				Expect(err).To(MatchError(ContainSubstring("layers exceed disk quota")))
			})

			Context("when the disk limit is zero", func() {
				It("doesn't fail", func() {
					fakeRemoteFetcher.ImageInfoReturns(image_puller.ImageInfo{
						LayersDigest: []image_puller.LayerDigest{
							image_puller.LayerDigest{Size: 1000},
							image_puller.LayerDigest{Size: 201},
						},
					}, nil)

					_, err := imagePuller.Pull(logger, groot.ImageSpec{
						ImageSrc:              remoteImageSrc,
						DiskLimit:             0,
						ExcludeImageFromQuota: false,
					})

					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when not including the image size in the limit", func() {
			It("doesn't fail", func() {
				fakeRemoteFetcher.ImageInfoReturns(image_puller.ImageInfo{
					LayersDigest: []image_puller.LayerDigest{
						image_puller.LayerDigest{Size: 1000},
						image_puller.LayerDigest{Size: 201},
					},
				}, nil)

				_, err := imagePuller.Pull(logger, groot.ImageSpec{
					ImageSrc:              remoteImageSrc,
					DiskLimit:             1024,
					ExcludeImageFromQuota: true,
				})

				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Context("when fetching the list of layers fails", func() {
		BeforeEach(func() {
			fakeRemoteFetcher.ImageInfoReturns(image_puller.ImageInfo{
				LayersDigest: []image_puller.LayerDigest{},
				Config:       specsv1.Image{},
			}, errors.New("failed to get list of layers"))
		})

		It("returns an error", func() {
			_, err := imagePuller.Pull(logger, groot.ImageSpec{
				ImageSrc: remoteImageSrc,
			})
			Expect(err).To(MatchError(ContainSubstring("failed to get list of layers")))
		})
	})

	Context("when UID and GID mappings are provided", func() {
		var spec groot.ImageSpec

		BeforeEach(func() {
			spec = groot.ImageSpec{
				ImageSrc: remoteImageSrc,
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
			_, err := imagePuller.Pull(logger, spec)
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

		It("appends a -namespaced suffix in all volume IDs", func() {
			_, err := imagePuller.Pull(logger, spec)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeVolumeDriver.PathCallCount()).To(Equal(3))
			_, chainID := fakeVolumeDriver.PathArgsForCall(0)
			Expect(chainID).To(Equal("chain-333-namespaced"))

			_, chainID = fakeVolumeDriver.PathArgsForCall(1)
			Expect(chainID).To(Equal("chain-222-namespaced"))

			_, chainID = fakeVolumeDriver.PathArgsForCall(2)
			Expect(chainID).To(Equal("layer-111-namespaced"))

			Expect(fakeVolumeDriver.CreateCallCount()).To(Equal(3))
			_, parentChainID, chainID := fakeVolumeDriver.CreateArgsForCall(0)
			Expect(parentChainID).To(BeEmpty())
			Expect(chainID).To(Equal("layer-111-namespaced"))

			_, parentChainID, chainID = fakeVolumeDriver.CreateArgsForCall(1)
			Expect(parentChainID).To(Equal("layer-111-namespaced"))
			Expect(chainID).To(Equal("chain-222-namespaced"))

			_, parentChainID, chainID = fakeVolumeDriver.CreateArgsForCall(2)
			Expect(parentChainID).To(Equal("chain-222-namespaced"))
			Expect(chainID).To(Equal("chain-333-namespaced"))
		})
	})

	Context("when all volumes exist", func() {
		BeforeEach(func() {
			fakeVolumeDriver.PathReturns("/path/to/volume", nil)
		})

		It("does not try to create any layer", func() {
			_, err := imagePuller.Pull(logger, groot.ImageSpec{
				ImageSrc: remoteImageSrc,
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
			_, err := imagePuller.Pull(logger, groot.ImageSpec{
				ImageSrc: remoteImageSrc,
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
			_, err := imagePuller.Pull(logger, groot.ImageSpec{
				ImageSrc: remoteImageSrc,
			})
			Expect(err).To(MatchError(ContainSubstring("failed to create volume")))
		})
	})

	Context("when streaming a blob fails", func() {
		BeforeEach(func() {
			fakeRemoteFetcher.StreamBlobReturns(nil, 0, errors.New("failed to stream blob"))
		})

		It("returns an error", func() {
			_, err := imagePuller.Pull(logger, groot.ImageSpec{ImageSrc: remoteImageSrc})
			Expect(err).To(MatchError(ContainSubstring("failed to stream blob")))
		})
	})

	Context("when unpacking a blob fails", func() {
		BeforeEach(func() {
			count := 0
			fakeUnpacker.UnpackStub = func(_ lager.Logger, _ image_puller.UnpackSpec) error {
				count++
				if count == 3 {
					return errors.New("failed to unpack the blob")
				}

				return nil
			}
		})

		It("returns an error", func() {
			_, err := imagePuller.Pull(logger, groot.ImageSpec{ImageSrc: remoteImageSrc})
			Expect(err).To(MatchError(ContainSubstring("failed to unpack the blob")))
		})

		It("deletes the volume", func() {
			_, err := imagePuller.Pull(logger, groot.ImageSpec{ImageSrc: remoteImageSrc})
			Expect(err).To(HaveOccurred())

			Expect(fakeVolumeDriver.DestroyVolumeCallCount()).To(Equal(1))
			_, path := fakeVolumeDriver.DestroyVolumeArgsForCall(0)
			Expect(path).To(Equal("chain-333"))
		})
	})
})

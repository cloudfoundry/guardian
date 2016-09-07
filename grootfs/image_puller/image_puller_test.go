package image_puller_test

import (
	"bytes"
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
		logger           lager.Logger
		fakeFetcher      *image_pullerfakes.FakeFetcher
		fakeImagePuller  *grootfakes.FakeImagePuller
		fakeUnpacker     *image_pullerfakes.FakeUnpacker
		fakeVolumeDriver *image_pullerfakes.FakeVolumeDriver
		expectedImgDesc  specsv1.Image

		imagePuller *image_puller.ImagePuller

		imageSrc *url.URL
	)

	BeforeEach(func() {
		fakeImagePuller = new(grootfakes.FakeImagePuller)

		fakeUnpacker = new(image_pullerfakes.FakeUnpacker)

		fakeFetcher = new(image_pullerfakes.FakeFetcher)
		expectedImgDesc = specsv1.Image{Author: "Groot"}
		fakeFetcher.ImageInfoReturns(
			image_puller.ImageInfo{
				LayersDigest: []image_puller.LayerDigest{
					image_puller.LayerDigest{BlobID: "i-am-a-layer", ChainID: "layer-111", ParentChainID: ""},
					image_puller.LayerDigest{BlobID: "i-am-another-layer", ChainID: "chain-222", ParentChainID: "layer-111"},
					image_puller.LayerDigest{BlobID: "i-am-the-last-layer", ChainID: "chain-333", ParentChainID: "chain-222"},
				},
				Config: expectedImgDesc,
			}, nil)

		fakeVolumeDriver = new(image_pullerfakes.FakeVolumeDriver)
		fakeVolumeDriver.PathReturns("", errors.New("volume does not exist"))

		imagePuller = image_puller.NewImagePuller(fakeFetcher, fakeUnpacker, fakeVolumeDriver)
		logger = lagertest.NewTestLogger("image-puller")

		var err error
		imageSrc, err = url.Parse("docker:///an/image")
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns the image description to the bundle spec", func() {
		bundle, err := imagePuller.Pull(logger, groot.ImageSpec{
			ImageSrc: imageSrc,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(bundle.Image).To(Equal(expectedImgDesc))
	})

	It("returns the last volume's path to the bundle spec", func() {
		fakeVolumeDriver.PathStub = func(_ lager.Logger, id string) (string, error) {
			return fmt.Sprintf("/path/to/volume/%s", id), nil
		}

		bundle, err := imagePuller.Pull(logger, groot.ImageSpec{
			ImageSrc: imageSrc,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(bundle.VolumePath).To(Equal("/path/to/volume/chain-333"))
	})

	Context("when fetching the list of layers fails", func() {
		BeforeEach(func() {
			fakeFetcher.ImageInfoReturns(image_puller.ImageInfo{
				LayersDigest: []image_puller.LayerDigest{},
				Config:       specsv1.Image{},
			}, errors.New("failed to get list of layers"))
		})

		It("returns an error", func() {
			_, err := imagePuller.Pull(logger, groot.ImageSpec{
				ImageSrc: imageSrc,
			})
			Expect(err).To(MatchError(ContainSubstring("failed to get list of layers")))
		})
	})

	It("creates volumes for all the layers", func() {
		_, err := imagePuller.Pull(logger, groot.ImageSpec{
			ImageSrc: imageSrc,
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
			ImageSrc: imageSrc,
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

	It("unpacks the layers got from the provided streamer", func() {
		fakeFetcher.StreamBlobStub = func(_ lager.Logger, imageURL *url.URL, source string) (io.ReadCloser, int64, error) {
			Expect(imageURL).To(Equal(imageSrc))

			stream := bytes.NewBuffer([]byte(fmt.Sprintf("layer-%s-contents", source)))
			return ioutil.NopCloser(stream), 0, nil
		}

		_, err := imagePuller.Pull(logger, groot.ImageSpec{
			ImageSrc: imageSrc,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeUnpacker.UnpackCallCount()).To(Equal(3))

		_, unpackSpec := fakeUnpacker.UnpackArgsForCall(0)
		contents, err := ioutil.ReadAll(unpackSpec.Stream)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("layer-i-am-a-layer-contents"))

		_, unpackSpec = fakeUnpacker.UnpackArgsForCall(1)
		contents, err = ioutil.ReadAll(unpackSpec.Stream)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("layer-i-am-another-layer-contents"))

		_, unpackSpec = fakeUnpacker.UnpackArgsForCall(2)
		contents, err = ioutil.ReadAll(unpackSpec.Stream)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("layer-i-am-the-last-layer-contents"))
	})

	Context("when UID and GID mappings are provided", func() {
		var spec groot.ImageSpec

		BeforeEach(func() {
			spec = groot.ImageSpec{
				ImageSrc: imageSrc,
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
			Expect(chainID).To(Equal("layer-111-namespaced"))

			_, chainID = fakeVolumeDriver.PathArgsForCall(1)
			Expect(chainID).To(Equal("chain-222-namespaced"))

			_, chainID = fakeVolumeDriver.PathArgsForCall(2)
			Expect(chainID).To(Equal("chain-333-namespaced"))

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

	Context("when a volume exists", func() {
		BeforeEach(func() {
			fakeVolumeDriver.PathReturns("/path/to/volume", nil)
		})

		It("does not try to create any layer", func() {
			_, err := imagePuller.Pull(logger, groot.ImageSpec{
				ImageSrc: imageSrc,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeVolumeDriver.CreateCallCount()).To(Equal(0))
		})
	})

	Context("when creating a volume fails", func() {
		BeforeEach(func() {
			fakeVolumeDriver.CreateReturns("", errors.New("failed to create volume"))
		})

		It("returns an error", func() {
			_, err := imagePuller.Pull(logger, groot.ImageSpec{
				ImageSrc: imageSrc,
			})
			Expect(err).To(MatchError(ContainSubstring("failed to create volume")))
		})
	})

	Context("when streaming a blob fails", func() {
		BeforeEach(func() {
			fakeFetcher.StreamBlobReturns(nil, 0, errors.New("failed to stream blob"))
		})

		It("returns an error", func() {
			_, err := imagePuller.Pull(logger, groot.ImageSpec{
				ImageSrc: imageSrc,
			})
			Expect(err).To(MatchError(ContainSubstring("failed to stream blob")))
		})
	})

	Context("when unpacking a blob fails", func() {
		BeforeEach(func() {
			fakeUnpacker.UnpackReturns(errors.New("failed to unpack the blob"))
		})

		It("returns an error", func() {
			_, err := imagePuller.Pull(logger, groot.ImageSpec{
				ImageSrc: imageSrc,
			})
			Expect(err).To(MatchError(ContainSubstring("failed to unpack the blob")))
		})
	})
})

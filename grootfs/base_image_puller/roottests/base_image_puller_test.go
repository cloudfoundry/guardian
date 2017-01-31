package base_image_puller_test

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"syscall"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/base_image_puller/base_image_pullerfakes"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/groot/grootfakes"
	"code.cloudfoundry.org/grootfs/store/storefakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("Base Image Puller", func() {
	var (
		logger                   lager.Logger
		fakeLocalFetcher         *base_image_pullerfakes.FakeFetcher
		fakeRemoteFetcher        *base_image_pullerfakes.FakeFetcher
		fakeBaseImagePuller      *grootfakes.FakeBaseImagePuller
		fakeUnpacker             *base_image_pullerfakes.FakeUnpacker
		fakeVolumeDriver         *storefakes.FakeVolumeDriver
		fakeDependencyRegisterer *base_image_pullerfakes.FakeDependencyRegisterer
		expectedImgDesc          specsv1.Image

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

		fakeVolumeDriver = new(storefakes.FakeVolumeDriver)
		fakeVolumeDriver.PathReturns("", errors.New("volume does not exist"))
		fakeVolumeDriver.CreateStub = func(_ lager.Logger, _, _ string) (string, error) {
			return ioutil.TempDir("", "volume")
		}

		fakeDependencyRegisterer = new(base_image_pullerfakes.FakeDependencyRegisterer)

		baseImagePuller = base_image_puller.NewBaseImagePuller(fakeLocalFetcher, fakeRemoteFetcher, fakeUnpacker, fakeVolumeDriver, fakeDependencyRegisterer)
		logger = lagertest.NewTestLogger("image-puller")

		var err error
		remoteBaseImageSrc, err = url.Parse("docker:///an/image")
		Expect(err).NotTo(HaveOccurred())

		fakeVolumeDriver.CreateStub = func(_ lager.Logger, _, chainID string) (string, error) {
			return ioutil.TempDir("", "volume")
		}
	})

	Describe("volumes ownership", func() {
		var spec groot.BaseImageSpec

		BeforeEach(func() {
			spec = groot.BaseImageSpec{
				BaseImageSrc: remoteBaseImageSrc,
			}
		})

		It("sets the ownership of the store to the spec's owner ids", func() {
			spec.OwnerUID = 10000
			spec.OwnerGID = 5000

			image, err := baseImagePuller.Pull(logger, spec)
			Expect(err).NotTo(HaveOccurred())

			Expect(image.VolumePath)
			volumePath, err := os.Stat(image.VolumePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(volumePath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(10000)))
			Expect(volumePath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(5000)))
		})

		Context("and both owner ids are 0", func() {
			It("doesn't enforce the ownership", func() {
				spec.OwnerUID = 0
				spec.OwnerGID = 0

				image, err := baseImagePuller.Pull(logger, spec)
				Expect(err).NotTo(HaveOccurred())

				Expect(image.VolumePath)
				volumePath, err := os.Stat(image.VolumePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(volumePath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(0)))
				Expect(volumePath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(0)))
			})
		})

		Context("and only owner uid mapping is 0", func() {
			It("enforces the ownership", func() {
				spec.OwnerUID = 0
				spec.OwnerGID = 5000

				image, err := baseImagePuller.Pull(logger, spec)
				Expect(err).NotTo(HaveOccurred())

				Expect(image.VolumePath)
				volumePath, err := os.Stat(image.VolumePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(volumePath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(0)))
				Expect(volumePath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(5000)))
			})
		})

		Context("and only owner gid mapping is 0", func() {
			It("enforces the ownership", func() {
				spec.OwnerUID = 10000
				spec.OwnerGID = 0

				image, err := baseImagePuller.Pull(logger, spec)
				Expect(err).NotTo(HaveOccurred())

				Expect(image.VolumePath)
				volumePath, err := os.Stat(image.VolumePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(volumePath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(10000)))
				Expect(volumePath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(0)))
			})
		})
	})
})

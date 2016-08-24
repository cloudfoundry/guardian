package cloner_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	"code.cloudfoundry.org/grootfs/cloner"
	"code.cloudfoundry.org/grootfs/cloner/clonerfakes"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/groot/grootfakes"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RemoteCloner", func() {
	var (
		logger           lager.Logger
		remote           *cloner.RemoteCloner
		fakeFetcher      *clonerfakes.FakeFetcher
		fakeStreamer     *clonerfakes.FakeStreamer
		fakeUnpacker     *clonerfakes.FakeUnpacker
		fakeVolumeDriver *grootfakes.FakeVolumeDriver
		bundle           *store.Bundle
	)

	BeforeEach(func() {
		bundle = store.NewBundle("/bundle/path")
		fakeFetcher = new(clonerfakes.FakeFetcher)
		fakeStreamer = new(clonerfakes.FakeStreamer)
		fakeFetcher.StreamerReturns(fakeStreamer, nil)
		fakeFetcher.LayersDigestReturns(
			[]cloner.LayerDigest{
				cloner.LayerDigest{BlobID: "i-am-a-layer", DiffID: "layer-111", ChainID: "layer-111", ParentChainID: ""},
				cloner.LayerDigest{BlobID: "i-am-another-layer", DiffID: "layer-222", ChainID: "chain-222", ParentChainID: "layer-111"},
				cloner.LayerDigest{BlobID: "i-am-the-last-layer", DiffID: "layer-333", ChainID: "chain-333", ParentChainID: "chain-222"},
			}, nil,
		)

		fakeVolumeDriver = new(grootfakes.FakeVolumeDriver)
		fakeVolumeDriver.PathReturns("", errors.New("volume does not exist"))

		fakeUnpacker = new(clonerfakes.FakeUnpacker)

		remote = cloner.NewRemoteCloner(fakeFetcher, fakeUnpacker, fakeVolumeDriver)
		logger = lagertest.NewTestLogger("remote-cloner")
	})

	Context("when passing an invalid URL", func() {
		It("returns an error", func() {
			Expect(remote.Clone(logger, groot.CloneSpec{
				Bundle: bundle,
				Image:  "%%!!#@!^&",
			})).To(MatchError(ContainSubstring("parsing URL")))
		})
	})

	Context("when fetching the list of layers fails", func() {
		BeforeEach(func() {
			fakeFetcher.LayersDigestReturns([]cloner.LayerDigest{}, errors.New("KABOM!"))
		})

		It("returns an error", func() {
			Expect(remote.Clone(logger, groot.CloneSpec{
				Bundle: bundle,
				Image:  "docker:///cfgarden/image",
			})).To(MatchError(ContainSubstring("KABOM!")))
		})
	})

	Context("when getting a streamer fails", func() {
		BeforeEach(func() {
			fakeFetcher.StreamerReturns(nil, errors.New("KABOM!"))
		})

		It("returns an error", func() {
			Expect(remote.Clone(logger, groot.CloneSpec{
				Bundle: bundle,
				Image:  "docker:///cfgarden/image",
			})).To(MatchError(ContainSubstring("KABOM!")))
		})
	})

	It("creates volumes for all the layers", func() {
		Expect(remote.Clone(logger, groot.CloneSpec{
			Bundle: bundle,
			Image:  "docker:///cfgarden/image",
		})).To(Succeed())

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

		Expect(remote.Clone(logger, groot.CloneSpec{
			Bundle: bundle,
			Image:  "docker:///cfgarden/image",
		})).To(Succeed())

		Expect(fakeUnpacker.UnpackCallCount()).To(Equal(3))
		_, unpackSpec := fakeUnpacker.UnpackArgsForCall(0)
		Expect(unpackSpec.TargetPath).To(Equal("/volume/layer-111"))
		_, unpackSpec = fakeUnpacker.UnpackArgsForCall(1)
		Expect(unpackSpec.TargetPath).To(Equal("/volume/chain-222"))
		_, unpackSpec = fakeUnpacker.UnpackArgsForCall(2)
		Expect(unpackSpec.TargetPath).To(Equal("/volume/chain-333"))
	})

	It("unpacks the layers got from the provided streamer", func() {
		fakeStreamer.StreamStub = func(_ lager.Logger, source string) (io.ReadCloser, int64, error) {
			stream := bytes.NewBuffer([]byte(fmt.Sprintf("layer-%s-contents", source)))
			return ioutil.NopCloser(stream), 0, nil
		}

		Expect(remote.Clone(logger, groot.CloneSpec{
			Bundle: bundle,
			Image:  "docker:///cfgarden/image",
		})).To(Succeed())

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
		var spec groot.CloneSpec

		BeforeEach(func() {
			spec = groot.CloneSpec{
				Bundle: bundle,
				Image:  "docker:///cfgarden/image",
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
			Expect(remote.Clone(logger, spec)).To(Succeed())

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
			Expect(remote.Clone(logger, spec)).To(Succeed())

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

			Expect(fakeVolumeDriver.SnapshotCallCount()).To(Equal(1))
			_, id, _ := fakeVolumeDriver.SnapshotArgsForCall(0)
			Expect(id).To(Equal("chain-333-namespaced"))
		})
	})

	Context("when a volume exists", func() {
		BeforeEach(func() {
			fakeVolumeDriver.PathReturns("/path/to/volume", nil)
		})

		It("does not try to create any layer", func() {
			Expect(remote.Clone(logger, groot.CloneSpec{
				Bundle: bundle,
				Image:  "docker:///cfgarden/image",
			})).To(Succeed())

			Expect(fakeVolumeDriver.CreateCallCount()).To(Equal(0))
		})
	})

	Context("when creating a volume fails", func() {
		BeforeEach(func() {
			fakeVolumeDriver.CreateReturns("", errors.New("KABOM!"))
		})

		It("returns an error", func() {
			Expect(remote.Clone(logger, groot.CloneSpec{
				Bundle: bundle,
				Image:  "docker:///cfgarden/image",
			})).To(MatchError(ContainSubstring("KABOM!")))
		})
	})

	Context("when streaming a blob fails", func() {
		BeforeEach(func() {
			fakeStreamer.StreamReturns(nil, 0, errors.New("KABOM!"))
		})

		It("returns an error", func() {
			Expect(remote.Clone(logger, groot.CloneSpec{
				Bundle: bundle,
				Image:  "docker:///cfgarden/image",
			})).To(MatchError(ContainSubstring("KABOM!")))
		})
	})

	Context("when unpacking a blob fails", func() {
		BeforeEach(func() {
			fakeUnpacker.UnpackReturns(errors.New("KABOM!"))
		})

		It("returns an error", func() {
			Expect(remote.Clone(logger, groot.CloneSpec{
				Bundle: bundle,
				Image:  "docker:///cfgarden/image",
			})).To(MatchError(ContainSubstring("KABOM!")))
		})
	})

	It("snapshots the last layer to the rootFSPath", func() {
		Expect(remote.Clone(logger, groot.CloneSpec{
			Bundle: bundle,
			Image:  "docker:///cfgarden/image",
		})).To(Succeed())

		Expect(fakeVolumeDriver.SnapshotCallCount()).To(Equal(1))

		_, id, targetPath := fakeVolumeDriver.SnapshotArgsForCall(0)
		Expect(id).To(Equal("chain-333"))
		Expect(targetPath).To(Equal("/bundle/path/rootfs"))
	})

	Context("when creating rootfs snapshot fails", func() {
		BeforeEach(func() {
			fakeVolumeDriver.SnapshotReturns(errors.New("KABOM!"))
		})

		It("returns an error", func() {
			Expect(remote.Clone(logger, groot.CloneSpec{
				Bundle: bundle,
				Image:  "docker:///cfgarden/image",
			})).To(MatchError(ContainSubstring("KABOM!")))
		})
	})
})

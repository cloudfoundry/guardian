package cloner_test

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net/url"

	"code.cloudfoundry.org/grootfs/cloner"
	"code.cloudfoundry.org/grootfs/cloner/clonerfakes"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RemoteCloner", func() {
	var (
		rootFSPath string
		image      string

		logger            lager.Logger
		remote            *cloner.RemoteCloner
		fakeRemoteFetcher *clonerfakes.FakeRemoteFetcher
		fakeStreamer      *clonerfakes.FakeStreamer
		fakeUnpacker      *clonerfakes.FakeUnpacker
	)

	BeforeEach(func() {
		var err error

		image = "docker:///cfgarden/image"

		rootFSPath, err = ioutil.TempDir("", "rootfs")
		Expect(err).NotTo(HaveOccurred())
		logger = lagertest.NewTestLogger("remote-cloner")
		Expect(err).NotTo(HaveOccurred())

		fakeRemoteFetcher = new(clonerfakes.FakeRemoteFetcher)
		fakeRemoteFetcher.LayersDigestStub = func(_ lager.Logger, _ *url.URL) ([]string, error) {
			return []string{
				"i-am-a-layer",
				"i-am-another-layer",
				"i-am-the-last-layer",
			}, nil
		}

		fakeStreamer = new(clonerfakes.FakeStreamer)
		fakeStreamer.StreamStub = func(_ lager.Logger, _ string) (io.ReadCloser, int64, error) {
			stream := bytes.NewBuffer([]byte("layer-contents"))
			return ioutil.NopCloser(stream), 0, nil
		}

		fakeRemoteFetcher.StreamerStub = func(_ lager.Logger, imageURL *url.URL) (cloner.Streamer, error) {
			Expect(imageURL.String()).To(Equal(image))
			return fakeStreamer, nil
		}

		fakeUnpacker = new(clonerfakes.FakeUnpacker)

		remote = cloner.NewRemoteCloner(fakeRemoteFetcher, fakeUnpacker)
	})

	It("fetches the list of blobs", func() {
		Expect(remote.Clone(logger, groot.CloneSpec{
			Image:      image,
			RootFSPath: rootFSPath,
		})).To(Succeed())

		Expect(fakeRemoteFetcher.LayersDigestCallCount()).To(Equal(1))
		_, imgURL := fakeRemoteFetcher.LayersDigestArgsForCall(0)
		Expect(imgURL.String()).To(Equal(image))
	})

	It("uses a streamer to stream the blobs", func() {
		Expect(remote.Clone(logger, groot.CloneSpec{
			Image:      image,
			RootFSPath: rootFSPath,
		})).To(Succeed())

		Expect(fakeRemoteFetcher.StreamerCallCount()).To(Equal(1))
		_, imgURL := fakeRemoteFetcher.StreamerArgsForCall(0)
		Expect(imgURL.String()).To(Equal(image))

		Expect(fakeStreamer.StreamCallCount()).To(Equal(3))
		_, layerA := fakeStreamer.StreamArgsForCall(0)
		Expect(layerA).To(Equal("i-am-a-layer"))
		_, layerB := fakeStreamer.StreamArgsForCall(1)
		Expect(layerB).To(Equal("i-am-another-layer"))
		_, layerC := fakeStreamer.StreamArgsForCall(2)
		Expect(layerC).To(Equal("i-am-the-last-layer"))
	})

	It("unpacks the blobs", func() {
		Expect(remote.Clone(logger, groot.CloneSpec{
			Image:      image,
			RootFSPath: rootFSPath,
		})).To(Succeed())

		Expect(fakeUnpacker.UnpackCallCount()).To(Equal(3))
		_, unpack := fakeUnpacker.UnpackArgsForCall(0)
		contents, err := ioutil.ReadAll(unpack.Stream)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("layer-contents"))
	})

	It("unpacks the blobs in the same rootfs path", func() {
		spec := groot.CloneSpec{
			Image:      image,
			RootFSPath: rootFSPath,
		}

		Expect(remote.Clone(logger, spec)).To(Succeed())

		Expect(fakeUnpacker.UnpackCallCount()).To(Equal(3))
		_, unpack := fakeUnpacker.UnpackArgsForCall(0)
		Expect(unpack.RootFSPath).To(Equal(rootFSPath))
	})

	It("applies the UID and GID mappings in the unpacked blobs", func() {
		spec := groot.CloneSpec{
			Image:      image,
			RootFSPath: rootFSPath,
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

		Expect(remote.Clone(logger, spec)).To(Succeed())

		Expect(fakeUnpacker.UnpackCallCount()).To(Equal(3))
		_, unpack := fakeUnpacker.UnpackArgsForCall(0)
		Expect(unpack.UIDMappings).To(Equal(spec.UIDMappings))
		Expect(unpack.GIDMappings).To(Equal(spec.GIDMappings))
	})

	Context("when passing an invalid URL", func() {
		It("returns an error", func() {
			Expect(remote.Clone(logger, groot.CloneSpec{
				Image:      "%%!!#@!^&",
				RootFSPath: rootFSPath,
			})).To(MatchError(ContainSubstring("parsing URL")))
		})
	})

	Context("when fetching the list of layers fails", func() {
		BeforeEach(func() {
			fakeRemoteFetcher.LayersDigestReturns([]string{}, errors.New("KABOM!"))
		})

		It("returns an error", func() {
			Expect(remote.Clone(logger, groot.CloneSpec{
				Image:      image,
				RootFSPath: rootFSPath,
			})).To(MatchError(ContainSubstring("KABOM!")))
		})
	})

	Context("when getting a streamer fails", func() {
		BeforeEach(func() {
			fakeRemoteFetcher.StreamerReturns(nil, errors.New("KABOM!"))
		})

		It("returns an error", func() {
			Expect(remote.Clone(logger, groot.CloneSpec{
				Image:      image,
				RootFSPath: rootFSPath,
			})).To(MatchError(ContainSubstring("KABOM!")))
		})
	})

	Context("when streaming a blob fails", func() {
		BeforeEach(func() {
			fakeStreamer.StreamReturns(nil, 0, errors.New("KABOM!"))
		})

		It("returns an error", func() {
			Expect(remote.Clone(logger, groot.CloneSpec{
				Image:      image,
				RootFSPath: rootFSPath,
			})).To(MatchError(ContainSubstring("KABOM!")))
		})
	})

	Context("when unpacking a blob fails", func() {
		BeforeEach(func() {
			fakeUnpacker.UnpackReturns(errors.New("KABOM!"))
		})

		It("returns an error", func() {
			Expect(remote.Clone(logger, groot.CloneSpec{
				Image:      image,
				RootFSPath: rootFSPath,
			})).To(MatchError(ContainSubstring("KABOM!")))
		})
	})
})

package cloner_test

import (
	"errors"
	"io/ioutil"
	"os"

	clonerpkg "code.cloudfoundry.org/grootfs/cloner"
	"code.cloudfoundry.org/grootfs/cloner/clonerfakes"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LocalCloner", func() {
	var (
		streamer *clonerfakes.FakeStreamer
		unpacker *clonerfakes.FakeUnpacker
		cloner   *clonerpkg.LocalCloner
		logger   lager.Logger
	)

	BeforeEach(func() {
		streamer = new(clonerfakes.FakeStreamer)
		unpacker = new(clonerfakes.FakeUnpacker)
		cloner = clonerpkg.NewLocalCloner(streamer, unpacker)
		logger = lagertest.NewTestLogger("cloner")
	})

	Describe("Clone", func() {
		It("reads the correct source", func() {
			Expect(cloner.Clone(logger, groot.CloneSpec{
				Image: "/someplace",
			})).To(Succeed())

			Expect(streamer.StreamCallCount()).To(Equal(1))
			_, source := streamer.StreamArgsForCall(0)
			Expect(source).To(Equal("/someplace"))
		})

		It("writes using the correct write spec", func() {
			uidMappings := []groot.IDMappingSpec{
				groot.IDMappingSpec{HostID: 1, NamespaceID: 2, Size: 10},
			}
			gidMappings := []groot.IDMappingSpec{
				groot.IDMappingSpec{HostID: 10, NamespaceID: 20, Size: 100},
			}

			Expect(cloner.Clone(logger, groot.CloneSpec{
				RootFSPath:  "/someplace",
				UIDMappings: uidMappings,
				GIDMappings: gidMappings,
			})).To(Succeed())

			Expect(unpacker.UnpackCallCount()).To(Equal(1))
			_, writeSpec := unpacker.UnpackArgsForCall(0)
			Expect(writeSpec.TargetPath).To(Equal("/someplace"))
			Expect(writeSpec.UIDMappings).To(Equal(uidMappings))
			Expect(writeSpec.GIDMappings).To(Equal(gidMappings))
		})

		It("pipes the streamer and the unpacker", func() {
			pipeR, pipeW, err := os.Pipe()
			Expect(err).ToNot(HaveOccurred())

			streamer.StreamReturns(pipeR, 0, nil)

			Expect(cloner.Clone(logger, groot.CloneSpec{})).To(Succeed())

			Expect(unpacker.UnpackCallCount()).To(Equal(1))
			_, writeSpec := unpacker.UnpackArgsForCall(0)

			_, err = pipeW.WriteString("hello-world")
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeW.Close()).To(Succeed())

			contents, err := ioutil.ReadAll(writeSpec.Stream)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(contents)).To(Equal("hello-world"))
		})

		Context("when the streamer fails", func() {
			It("returns an error", func() {
				streamer.StreamReturns(nil, 0, errors.New("cannot read"))

				Expect(
					cloner.Clone(logger, groot.CloneSpec{}),
				).To(MatchError(ContainSubstring("cannot read")))
			})
		})

		Context("when the unpacker fails", func() {
			It("returns an error", func() {
				unpacker.UnpackReturns(errors.New("cannot write"))

				Expect(
					cloner.Clone(logger, groot.CloneSpec{}),
				).To(MatchError(ContainSubstring("cannot write")))
			})
		})
	})
})

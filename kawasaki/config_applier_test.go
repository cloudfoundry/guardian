package kawasaki_test

import (
	"errors"
	"io/ioutil"
	"os"

	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConfigApplier", func() {
	var (
		fakeHostConfigApplier      *fakes.FakeHostApplier
		fakeContainerConfigApplier *fakes.FakeContainerApplier
		fakeNsExecer               *fakes.FakeNetnsExecer

		netnsFD *os.File

		applier kawasaki.ConfigApplier
	)

	BeforeEach(func() {
		fakeHostConfigApplier = new(fakes.FakeHostApplier)
		fakeContainerConfigApplier = new(fakes.FakeContainerApplier)

		fakeNsExecer = new(fakes.FakeNetnsExecer)

		var err error
		netnsFD, err = ioutil.TempFile("", "")
		Expect(err).NotTo(HaveOccurred())

		applier = kawasaki.NewConfigApplier(fakeHostConfigApplier, fakeContainerConfigApplier, fakeNsExecer)
	})

	AfterEach(func() {
		Expect(os.Remove(netnsFD.Name())).To(Succeed())
	})

	Describe("Apply", func() {
		Context("when the ns path cannot be opened", func() {
			It("returns an error", func() {
				err := applier.Apply(kawasaki.NetworkConfig{}, "DOESNOTEXIST")
				Expect(err).To(HaveOccurred())
			})

			It("does not configure anything", func() {
				applier.Apply(kawasaki.NetworkConfig{}, "DOESNOTEXIST")
				Expect(fakeHostConfigApplier.ApplyCallCount()).To(Equal(0))
			})
		})

		It("applies the configuration in the host", func() {
			cfg := kawasaki.NetworkConfig{
				ContainerIntf: "banana",
			}

			Expect(applier.Apply(cfg, netnsFD.Name())).To(Succeed())

			Expect(fakeHostConfigApplier.ApplyCallCount()).To(Equal(1))
			appliedCfg, fd := fakeHostConfigApplier.ApplyArgsForCall(0)
			Expect(appliedCfg).To(Equal(cfg))
			Expect(fd.Name()).To(Equal(netnsFD.Name()))
		})

		Context("if applying the host config fails", func() {
			It("returns the error", func() {
				fakeHostConfigApplier.ApplyReturns(errors.New("boom"))
				Expect(applier.Apply(kawasaki.NetworkConfig{}, netnsFD.Name())).To(MatchError("boom"))
			})
		})

		It("calls the namespace execer and applies the configuration in the container", func() {
			cfg := kawasaki.NetworkConfig{
				ContainerIntf: "banana",
			}

			Expect(applier.Apply(cfg, netnsFD.Name())).To(Succeed())

			Expect(fakeNsExecer.ExecCallCount()).To(Equal(1))
			fd, cb := fakeNsExecer.ExecArgsForCall(0)
			Expect(fd.Name()).To(Equal(netnsFD.Name()))

			Expect(fakeContainerConfigApplier.ApplyCallCount()).To(Equal(0))
			cb()
			Expect(fakeContainerConfigApplier.ApplyCallCount()).To(Equal(1))
			Expect(fakeContainerConfigApplier.ApplyArgsForCall(0)).To(Equal(cfg))
		})

		Context("if entering the namespace fails", func() {
			It("returns the error", func() {
				fakeNsExecer.ExecReturns(errors.New("boom"))
				Expect(applier.Apply(kawasaki.NetworkConfig{}, netnsFD.Name())).To(MatchError("boom"))
			})
		})

		Context("if container configuration fails", func() {
			It("returns the error", func() {
				fakeNsExecer.ExecStub = func(_ *os.File, cb func() error) error {
					return cb()
				}

				fakeContainerConfigApplier.ApplyReturns(errors.New("banana"))
				Expect(applier.Apply(kawasaki.NetworkConfig{}, netnsFD.Name())).To(MatchError("banana"))
			})
		})
	})
})

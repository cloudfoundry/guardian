package runcontainerd_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/guardian/rundmc/goci"
	. "code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/runcontainerdfakes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Rundmc/Runcontainerd/Bundleloader", func() {
	var (
		bundleLoader BndlLoader
		logger       lager.Logger
		nerd         *runcontainerdfakes.FakeContainerManager
		bndl         goci.Bndl
		bndlErr      error
	)

	BeforeEach(func() {
		nerd = new(runcontainerdfakes.FakeContainerManager)
		bundleLoader = NewBndlLoader(nerd)
		logger = lagertest.NewTestLogger("banana")
	})

	Describe("Load", func() {
		var (
			spec *specs.Spec
		)

		BeforeEach(func() {
			spec = &specs.Spec{Version: "the-coolest-potato"}
			nerd.SpecReturns(spec, nil)
		})

		JustBeforeEach(func() {
			bndl, bndlErr = bundleLoader.Load(logger, "container-handle")
		})

		It("gets the container spec", func() {
			Expect(nerd.SpecCallCount()).To(Equal(1))
			_, actualContainerID := nerd.SpecArgsForCall(0)
			Expect(actualContainerID).To(Equal("container-handle"))
		})

		It("returns a bundle with the spec", func() {
			Expect(bndl).To(Equal(goci.Bndl{Spec: *spec}))
		})

		Context("when getting the spec fails", func() {
			BeforeEach(func() {
				nerd.SpecReturns(nil, errors.New("no-spec-for-you"))
			})

			It("propagates the error", func() {
				Expect(bndlErr).To(MatchError("no-spec-for-you"))
			})
		})
	})
})

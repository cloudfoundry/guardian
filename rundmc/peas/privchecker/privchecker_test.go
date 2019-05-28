package privchecker_test

import (
	"errors"

	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/peas/privchecker"
	"code.cloudfoundry.org/guardian/rundmc/peas/privchecker/privcheckerfakes"
	"code.cloudfoundry.org/guardian/rundmc/runrunc/runruncfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("privchecker", func() {
	var (
		privilegedContainerId      = "container-id"
		privilegedConfigPath       = "/config/path"
		fakeBundleLoader           *runruncfakes.FakeBundleLoader
		fakeDepot                  *privcheckerfakes.FakeDepot
		bundleWithoutUserNamespace goci.Bndl
		bundleWithUserNamespace    goci.Bndl
		privChecker                *privchecker.PrivilegeChecker
	)

	BeforeEach(func() {
		fakeBundleLoader = new(runruncfakes.FakeBundleLoader)
		fakeDepot = new(privcheckerfakes.FakeDepot)
		fakeDepot.LookupReturns(privilegedConfigPath, nil)
		bundleWithoutUserNamespace = goci.Bndl{
			Spec: specs.Spec{
				Linux: &specs.Linux{},
			},
		}
		bundleWithUserNamespace = goci.Bndl{
			Spec: specs.Spec{
				Linux: &specs.Linux{
					Namespaces: []specs.LinuxNamespace{
						specs.LinuxNamespace{
							Type: "user",
						},
					},
				},
			},
		}

		privChecker = &privchecker.PrivilegeChecker{
			BundleLoader: fakeBundleLoader,
			Depot:        fakeDepot,
		}
	})

	Context("when there is user namespace defined in the bundle", func() {
		BeforeEach(func() {
			fakeBundleLoader.LoadReturns(bundleWithUserNamespace, nil)
		})

		It("reports bundle as unprivileged", func() {
			Expect(privChecker.Privileged(privilegedContainerId)).To(BeFalse())

			Expect(fakeDepot.LookupCallCount()).To(Equal(1))
			_, actualId := fakeDepot.LookupArgsForCall(0)
			Expect(actualId).To(Equal(privilegedContainerId))

			Expect(fakeBundleLoader.LoadCallCount()).To(Equal(1))
			actualBundlePath := fakeBundleLoader.LoadArgsForCall(0)
			Expect(actualBundlePath).To(Equal(privilegedConfigPath))
		})
	})

	Context("when there is no user namespace defined in the bundle", func() {
		BeforeEach(func() {
			fakeBundleLoader.LoadReturns(bundleWithoutUserNamespace, nil)
		})

		It("report bundle as privileged", func() {
			Expect(privChecker.Privileged(privilegedConfigPath)).To(BeTrue())
		})
	})

	Context("when Depot returns an error", func() {
		BeforeEach(func() {
			fakeDepot.LookupReturns("", errors.New("lookup-error"))
		})

		It("errors", func() {
			_, err := privChecker.Privileged("random/path")
			Expect(err).To(MatchError(ContainSubstring("lookup-error")))
		})
	})

	Context("when BundleLoader returns an error", func() {
		BeforeEach(func() {
			fakeBundleLoader.LoadReturns(goci.Bndl{}, errors.New("load-error"))
		})

		It("errors", func() {
			_, err := privChecker.Privileged("random/path")
			Expect(err).To(MatchError(ContainSubstring("load-error")))
		})
	})
})

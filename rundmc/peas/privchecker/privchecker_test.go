package privchecker_test

import (
	"errors"

	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/peas/privchecker"
	"code.cloudfoundry.org/guardian/rundmc/peas/privchecker/privcheckerfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("privchecker", func() {
	var (
		privilegedContainerId      = "container-id"
		privilegedConfigPath       = "/config/path"
		fakeBundleLoader           *privcheckerfakes.FakeBundleLoader
		bundleWithoutUserNamespace goci.Bndl
		bundleWithUserNamespace    goci.Bndl
		privChecker                *privchecker.PrivilegeChecker
	)

	BeforeEach(func() {
		fakeBundleLoader = new(privcheckerfakes.FakeBundleLoader)
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
		}
	})

	Context("when there is user namespace defined in the bundle", func() {
		BeforeEach(func() {
			fakeBundleLoader.LoadReturns(bundleWithUserNamespace, nil)
		})

		It("reports bundle as unprivileged", func() {
			Expect(privChecker.Privileged(privilegedContainerId)).To(BeFalse())

			Expect(fakeBundleLoader.LoadCallCount()).To(Equal(1))
			_, actualId := fakeBundleLoader.LoadArgsForCall(0)
			Expect(actualId).To(Equal(privilegedContainerId))
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

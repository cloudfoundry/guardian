package privchecker_test

import (
	"errors"

	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/peas/privchecker"
	"code.cloudfoundry.org/guardian/rundmc/runrunc/runruncfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("privchecker", func() {
	var (
		privilegedConfigPath string
		fakeBundleLoader     *runruncfakes.FakeBundleLoader
		bundle               goci.Bndl
		privChecker          *privchecker.PrivilegeChecker
	)

	BeforeEach(func() {
		fakeBundleLoader = new(runruncfakes.FakeBundleLoader)
		bundle = goci.Bndl{
			Spec: specs.Spec{
				Linux: &specs.Linux{},
			},
		}

		privChecker = &privchecker.PrivilegeChecker{
			BundleLoader: fakeBundleLoader,
		}
	})

	Context("when there is user namespace defined in the bundle", func() {
		BeforeEach(func() {
			bundle.Spec.Linux.Namespaces = []specs.LinuxNamespace{
				specs.LinuxNamespace{
					Type: "user",
				},
			}
			fakeBundleLoader.LoadReturns(bundle, nil)
		})

		It("reports bundle as unprivileged", func() {
			Expect(privChecker.Privileged(privilegedConfigPath)).To(BeFalse())
		})
	})

	Context("when there is no user namespace defined in the bundle", func() {
		BeforeEach(func() {
			fakeBundleLoader.LoadReturns(bundle, nil)
		})

		It("report bundle as privileged", func() {
			privchecker := &privchecker.PrivilegeChecker{
				BundleLoader: fakeBundleLoader,
			}
			Expect(privchecker.Privileged(privilegedConfigPath)).To(BeTrue())
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

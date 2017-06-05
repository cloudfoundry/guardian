package runrunc_test

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("WindowsExecPreparer", func() {
	It("returns a PreparedSpec containing the executable and args", func() {
		execPreparer := runrunc.WindowsExecPreparer{}
		procSpec := garden.ProcessSpec{Path: "foo", Args: []string{"bar", "baz"}}
		prepSpec, err := execPreparer.Prepare(lagertest.NewTestLogger("windows-exec-preparer"), "", procSpec)
		Expect(err).NotTo(HaveOccurred())
		Expect(prepSpec).To(Equal(&runrunc.PreparedSpec{
			Process: specs.Process{
				Args: []string{"foo", "bar", "baz"},
			},
		}))
	})
})

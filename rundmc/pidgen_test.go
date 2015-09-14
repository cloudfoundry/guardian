package rundmc_test

import (
	"github.com/cloudfoundry-incubator/guardian/rundmc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pid Generation", func() {
	It("generates sequential pids", func() {
		p := &rundmc.SimplePidGenerator{}
		Expect(p.Generate()).To(BeEquivalentTo(1))
		Expect(p.Generate()).To(BeEquivalentTo(2))
	})
})

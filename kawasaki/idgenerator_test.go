package kawasaki_test

import (
	"code.cloudfoundry.org/guardian/kawasaki"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Generate", func() {
	It("generates an 11 character ID", func() {
		idgen := kawasaki.NewSequentialIDGenerator(0)
		Expect(idgen.Generate()).To(HaveLen(11))
	})

	It("generates unique IDs on each invocation", func() {
		idgen := kawasaki.NewSequentialIDGenerator(0)
		id1 := idgen.Generate()
		id2 := idgen.Generate()

		Expect(id1).NotTo(Equal(id2))
	})

	It("generates deterministic IDs based on the seed", func() {
		idgen := kawasaki.NewSequentialIDGenerator(0)
		id1 := idgen.Generate()
		id2 := idgen.Generate()

		idgen = kawasaki.NewSequentialIDGenerator(1)
		id3 := idgen.Generate()

		Expect(id1).NotTo(Equal(id2))
		Expect(id3).To(Equal(id2))
	})
})

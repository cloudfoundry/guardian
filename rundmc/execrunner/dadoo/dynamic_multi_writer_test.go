package dadoo_test

import (
	"bytes"
	"fmt"

	"code.cloudfoundry.org/guardian/rundmc/execrunner/dadoo"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DynamicMultiWriter", func() {
	It("writes the output to all attached writers", func() {
		var buf1, buf2 bytes.Buffer

		multiW := dadoo.NewDynamicMultiWriter()

		fmt.Fprint(multiW, "hello no writers")

		multiW.Attach(&buf1)
		fmt.Fprint(multiW, "hello one writer")

		multiW.Attach(&buf2)
		fmt.Fprint(multiW, " hello both writers")

		Expect(buf1.String()).To(Equal("hello one writer hello both writers"))
		Expect(buf2.String()).To(Equal(" hello both writers"))
	})

	It("counts the number of attached writers", func() {
		var buf1, buf2 bytes.Buffer

		multiW := dadoo.NewDynamicMultiWriter()
		multiW.Attach(&buf1)
		multiW.Attach(&buf2)

		Expect(multiW.Count()).To(Equal(2))
	})
})

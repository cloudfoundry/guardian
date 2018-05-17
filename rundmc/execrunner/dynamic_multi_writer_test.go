package execrunner_test

import (
	"bytes"
	"fmt"
	"sync"

	"code.cloudfoundry.org/guardian/rundmc/execrunner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DynamicMultiWriter", func() {
	It("writes the output to all attached writers", func() {
		var buf1, buf2 bytes.Buffer

		multiW := execrunner.NewDynamicMultiWriter()

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

		multiW := execrunner.NewDynamicMultiWriter()
		multiW.Attach(&buf1)
		multiW.Attach(&buf2)

		Expect(multiW.Count()).To(Equal(2))
	})

	It("always attaches the correct number of attached writers", func() {
		var buf bytes.Buffer
		lock := sync.Mutex{}
		found := []int{}
		expected := []int{}
		wg := sync.WaitGroup{}

		multiW := execrunner.NewDynamicMultiWriter()

		for i := 1; i < 101; i++ {
			expected = append(expected, i)
		}

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer GinkgoRecover()

				count := multiW.Attach(&buf)
				lock.Lock()
				found = append(found, count)
				lock.Unlock()
			}()
		}

		wg.Wait()

		Expect(found).To(ConsistOf(expected))
	})
})

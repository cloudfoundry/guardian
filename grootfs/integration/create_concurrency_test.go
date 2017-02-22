package integration_test

import (
	"fmt"
	"sync"

	"code.cloudfoundry.org/grootfs/groot"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Concurrent creations", func() {
	BeforeEach(func() {
		// run this to setup the store before concurrency!
		_, err := Runner.Create(groot.CreateSpec{
			ID:        "test-pre-warm",
			BaseImage: "docker:///cfgarden/empty",
		})
		Expect(err).NotTo(HaveOccurred())
	})

	It("can create multiple rootfses of the same image concurrently", func() {
		wg := new(sync.WaitGroup)

		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(wg *sync.WaitGroup, idx int) {
				defer GinkgoRecover()
				defer wg.Done()

				_, err := Runner.Create(groot.CreateSpec{
					ID:        fmt.Sprintf("test-%d", idx),
					BaseImage: "docker:///cfgarden/empty",
				})
				Expect(err).NotTo(HaveOccurred())
			}(wg, i)
		}

		wg.Wait()
	})

	Describe("parallel create and clean", func() {
		It("works in parallel, without errors", func() {
			wg := new(sync.WaitGroup)

			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()

				for i := 0; i < 3; i++ {
					_, err := Runner.Create(groot.CreateSpec{
						ID:        fmt.Sprintf("test-%d", i),
						BaseImage: "docker:///cfgarden/empty",
					})
					Expect(err).NotTo(HaveOccurred())
				}
			}()

			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()

				_, err := Runner.Clean(0, []string{})
				Expect(err).To(Succeed())
			}()

			wg.Wait()
		})
	})
})

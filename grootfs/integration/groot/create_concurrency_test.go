package groot_test

import (
	"fmt"
	"sync"

	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
)

var _ = Describe("Concurrent creations", func() {
	It("can create multiple rootfses of the same image concurrently", func() {
		wg := new(sync.WaitGroup)

		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(wg *sync.WaitGroup, idx int) {
				defer GinkgoRecover()
				defer wg.Done()

				integration.CreateBundle(
					GrootFSBin, StorePath, "docker:///cfgarden/empty",
					fmt.Sprintf("test-%d", idx), 0,
				)
			}(wg, i)
		}

		wg.Wait()
	})
})

package groot_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Concurrent creations", func() {
	var imagePath string

	BeforeEach(func() {
		var err error
		imagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(imagePath)).To(Succeed())
	})

	It("can create multiple rootfses of the same image concurrently", func() {
		// run this to setup the store before concurrency!
		integration.CreateBundle(GrootFSBin, StorePath, DraxBin, imagePath, "test-pre-warm", 0)

		wg := new(sync.WaitGroup)

		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(wg *sync.WaitGroup, idx int) {
				defer GinkgoRecover()
				defer wg.Done()

				integration.CreateBundle(
					GrootFSBin, StorePath, DraxBin, "docker:///cfgarden/empty",
					fmt.Sprintf("test-%d", idx), 0,
				)
			}(wg, i)
		}

		wg.Wait()
	})
})

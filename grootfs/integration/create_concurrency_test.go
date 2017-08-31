package integration_test

import (
	"fmt"
	"os"
	"sync"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Concurrent creations", func() {
	var workDir string

	BeforeEach(func() {
		err := Runner.RunningAsUser(0, 0).InitStore(runner.InitSpec{
			UIDMappings: []groot.IDMappingSpec{
				{HostID: GrootUID, NamespaceID: 0, Size: 1},
				{HostID: 100000, NamespaceID: 1, Size: 65000},
			},
			GIDMappings: []groot.IDMappingSpec{
				{HostID: GrootGID, NamespaceID: 0, Size: 1},
				{HostID: 100000, NamespaceID: 1, Size: 65000},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		workDir, err = os.Getwd()
		Expect(err).NotTo(HaveOccurred())

		Runner = Runner.SkipInitStore()
	})

	It("can create multiple rootfses of the same image concurrently", func() {
		wg := new(sync.WaitGroup)

		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(wg *sync.WaitGroup, idx int) {
				defer GinkgoRecover()
				defer wg.Done()
				runner := Runner.WithLogLevel(lager.ERROR) // clone runner to avoid data-race on stdout
				_, err := runner.Create(groot.CreateSpec{
					ID:                        fmt.Sprintf("test-%d", idx),
					BaseImage:                 fmt.Sprintf("oci://%s/assets/oci-test-image/grootfs-busybox:latest", workDir),
					Mount:                     mountByDefault(),
					DiskLimit:                 2*1024*1024 + 512*1024,
					ExcludeBaseImageFromQuota: true,
				})
				Expect(err).NotTo(HaveOccurred())
			}(wg, i)
		}

		wg.Wait()
	})

	It("work in parallel with clean", func() {
		wg := new(sync.WaitGroup)

		wg.Add(1)
		go func() {
			defer GinkgoRecover()
			defer wg.Done()

			for i := 0; i < 100; i++ {
				runner := Runner.WithLogLevel(lager.ERROR) // clone runner to avoid data-race on stdout
				_, err := runner.Create(groot.CreateSpec{
					ID:        fmt.Sprintf("test-%d", i),
					BaseImage: fmt.Sprintf("oci://%s/assets/oci-test-image/grootfs-busybox:latest", workDir),
					Mount:     mountByDefault(),
					DiskLimit: 2*1024*1024 + 512*1024,
				})
				Expect(err).NotTo(HaveOccurred())
			}
		}()

		wg.Add(1)
		go func() {
			defer GinkgoRecover()
			defer wg.Done()

			for i := 0; i < 100; i++ {
				runner := Runner.WithLogLevel(lager.ERROR) // clone runner to avoid data-race on stdout
				_, err := runner.Clean(0, []string{})
				Expect(err).To(Succeed())
			}
		}()

		wg.Wait()
	})
})

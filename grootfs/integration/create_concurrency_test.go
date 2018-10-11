package integration_test

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/grootfs/testhelpers"
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
					ID:                          fmt.Sprintf("test-%d-%d", GinkgoParallelNode(), idx),
					BaseImageURL:                integration.String2URL(fmt.Sprintf("oci://%s/assets/oci-test-image/grootfs-busybox:latest", workDir)),
					Mount:                       mountByDefault(),
					DiskLimit:                   2*1024*1024 + 512*1024,
					ExcludeBaseImageFromQuota:   true,
					CleanOnCreate:               true,
					CleanOnCreateThresholdBytes: 0,
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
					ID:           fmt.Sprintf("test-%d-%d", GinkgoParallelNode(), i),
					BaseImageURL: integration.String2URL(fmt.Sprintf("oci://%s/assets/oci-test-image/grootfs-busybox:latest", workDir)),
					Mount:        mountByDefault(),
					DiskLimit:    2*1024*1024 + 512*1024,
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
				_, err := runner.Clean(0)
				Expect(err).To(Succeed())
			}
		}()

		wg.Wait()
	})

	Context("when a creation is slow", func() {
		var (
			fastRegistry *testhelpers.FakeRegistry
			slowRegistry *testhelpers.FakeRegistry
		)

		createWithRegistry := func(registryAddr, imageId, imagePath string) error {
			runner := Runner.WithLogLevel(lager.ERROR).WithInsecureRegistry(registryAddr)
			_, err := runner.Create(groot.CreateSpec{
				ID:           fmt.Sprintf("test-%d-%s", GinkgoParallelNode(), imageId),
				BaseImageURL: integration.String2URL(fmt.Sprintf("docker://%s/%s", registryAddr, imagePath)),
			})
			return err
		}

		BeforeEach(func() {
			dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
			Expect(err).NotTo(HaveOccurred())
			fastRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)
			fastRegistry.Start()
			slowRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)
			slowRegistry.Start()

			slowRegistry.WhenGettingBlob(testhelpers.EmptyBaseImageV011.Layers[0].BlobID, 0, func(rw http.ResponseWriter, req *http.Request) {
				err := slowRegistry.Delay(30 * time.Second)
				if err == nil {
					slowRegistry.DelegateToActualRegistry(rw, req)
				}
			})
		})

		AfterEach(func() {
			fastRegistry.Stop()
			slowRegistry.Stop()
		})

		It("doesn't prevent other creations from running", func() {
			slowCreateDone := make(chan struct{})

			go func() {
				defer GinkgoRecover()
				defer func() {
					slowCreateDone <- struct{}{}
					close(slowCreateDone)
				}()
				createWithRegistry(slowRegistry.Addr(), "long-running", "cfgarden/empty:v0.1.1")
			}()

			time.Sleep(time.Second)
			Expect(createWithRegistry(fastRegistry.Addr(), "fast-running", "cfgarden/empty:schemaV1")).To(Succeed())

			select {
			case <-slowCreateDone:
				Fail("Slow create completed before the fast one!")
			default:
			}
		})
	})
})

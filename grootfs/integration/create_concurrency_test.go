package integration_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Concurrent creations", func() {
	Context("warm cache", func() {
		BeforeEach(func() {
			integration.SkipIfNonRootAndNotBTRFS(GrootfsTestUid, Driver)
			// run this to setup the store before concurrency!
			_, err := Runner.Create(groot.CreateSpec{
				ID:        "test-pre-warm",
				BaseImage: "docker:///cfgarden/empty",
				Mount:     true,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("can create multiple rootfses of the same image concurrently", func() {
			wg := new(sync.WaitGroup)

			for i := 0; i < 200; i++ {
				wg.Add(1)
				go func(wg *sync.WaitGroup, idx int) {
					defer GinkgoRecover()
					defer wg.Done()

					image, err := Runner.Create(groot.CreateSpec{
						ID:        fmt.Sprintf("test-%d", idx),
						BaseImage: "docker:///cfgarden/empty",
						Mount:     true,
						DiskLimit: 2*1024*1024 + 512*1024,
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(writeMegabytes(filepath.Join(image.Rootfs, "hello"), 2)).To(Succeed())
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

					for i := 0; i < 100; i++ {
						image, err := Runner.Create(groot.CreateSpec{
							ID:        fmt.Sprintf("test-%d", i),
							BaseImage: "docker:///cfgarden/empty",
							Mount:     true,
							DiskLimit: 2*1024*1024 + 512*1024,
						})
						Expect(err).NotTo(HaveOccurred())
						Expect(writeMegabytes(filepath.Join(image.Rootfs, "hello"), 2)).To(Succeed())
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

	Context("cold cache", func() {
		It("can create multiple rootfses of the same image concurrently", func() {
			wg := new(sync.WaitGroup)

			start := time.Now()
			for i := 0; i < 20; i++ {
				wg.Add(1)
				go func(wg *sync.WaitGroup, idx int) {
					defer GinkgoRecover()
					defer wg.Done()

					image, err := Runner.Create(groot.CreateSpec{
						ID:                        fmt.Sprintf("test-%d", idx),
						BaseImage:                 "docker:///node",
						Mount:                     true,
						DiskLimit:                 2*1024*1024 + 512*1024,
						ExcludeBaseImageFromQuota: true,
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(writeMegabytes(filepath.Join(image.Rootfs, "hello"), 2)).To(Succeed())
				}(wg, i)
			}

			wg.Wait()

			fmt.Fprintf(os.Stderr, "-> DURATION: %fs", time.Since(start).Seconds)
		})
	})
})

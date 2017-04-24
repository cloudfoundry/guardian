package ids_test

import (
	"os"
	"path/filepath"
	"sync"

	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs/tardis/ids"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IdDiscoverer", func() {
	var (
		discoverer *ids.Discoverer
		idDirPath  string
	)

	BeforeEach(func() {
		idDirPath = filepath.Join(StorePath, overlayxfs.IDDir)
		discoverer = ids.NewDiscoverer(idDirPath)

		Expect(os.MkdirAll(StorePath, 0777)).To(Succeed())
		Expect(os.MkdirAll(idDirPath, 0777)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(filepath.Join(StorePath, overlayxfs.IDDir))).To(Succeed())
	})

	Describe("Alloc", func() {
		Context("when the id dir is empty", func() {
			It("allocates the first available number", func() {
				id, err := discoverer.Alloc()
				Expect(err).NotTo(HaveOccurred())
				Expect(id).To(Equal(uint32(1)))
			})

			It("always allocates unique numbers", func() {
				id, err := discoverer.Alloc()
				Expect(err).NotTo(HaveOccurred())
				Expect(id).To(Equal(uint32(1)))

				id, err = discoverer.Alloc()
				Expect(err).NotTo(HaveOccurred())
				Expect(id).To(Equal(uint32(2)))

				id, err = discoverer.Alloc()
				Expect(err).NotTo(HaveOccurred())
				Expect(id).To(Equal(uint32(3)))
			})

			It("can be ran in parallel, without colisions", func() {
				concurrency := 1000
				ids := make([]int, concurrency)
				wg := sync.WaitGroup{}

				wg.Add(concurrency)
				for i := 0; i < concurrency; i++ {
					go func(i int) {
						defer GinkgoRecover()
						defer wg.Done()

						id, err := discoverer.Alloc()
						Expect(err).NotTo(HaveOccurred())
						ids[i] = int(id)
					}(i)
				}

				wg.Wait()
				Expect(Duplicates(ids)).To(BeEmpty())
			})
		})

		Context("when there's an error reading the ids dir", func() {
			BeforeEach(func() {
				Expect(os.Remove(idDirPath)).To(Succeed())
			})

			It("returns an error", func() {
				_, err := discoverer.Alloc()
				Expect(err).To(MatchError(ContainSubstring("reading directory")))
			})
		})
	})
})

func Duplicates(input []int) []int {
	u := make([]int, 0, len(input))
	m := make(map[int]bool)

	for _, val := range input {
		if _, ok := m[val]; !ok {
			m[val] = true
		} else {
			u = append(u, val)
		}
	}

	return u
}

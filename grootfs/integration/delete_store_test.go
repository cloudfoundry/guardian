package integration_test

import (
	"os"
	"path/filepath"

	grootfsRunner "code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/grootfs/testhelpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Delete Store", func() {
	var (
		runner    grootfsRunner.Runner
		spec      grootfsRunner.InitSpec
		storePath string
	)
	BeforeEach(func() {
		tmpDir, err := os.MkdirTemp("", "")
		Expect(err).NotTo(HaveOccurred())

		storePath = filepath.Join(tmpDir, "store")
		spec.StoreSizeBytes = 500 * 1024 * 1024

		runner = Runner.WithStore(storePath)
	})

	It("empties, unmounts and completely removes the given store path", func() {
		Expect(runner.InitStore(spec)).To(Succeed())
		Expect(storePath).To(BeAnExistingFile())

		Expect(testhelpers.XFSMountPoints()).To(ContainElement(storePath))

		storeContents, err := os.ReadDir(storePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(storeContents).ToNot(BeEmpty())

		Expect(runner.DeleteStore()).To(Succeed())

		Expect(storePath).ToNot(BeAnExistingFile())

		Expect(testhelpers.XFSMountPoints()).NotTo(ContainElement(storePath))
	})

	Context("when given a path which does not look like a store", func() {
		JustBeforeEach(func() {
			Expect(os.MkdirAll(storePath, 0600)).To(Succeed())
			storeContents, err := os.ReadDir(storePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(storeContents).To(BeEmpty())
		})

		It("does not delete the directory", func() {
			Expect(runner.DeleteStore()).NotTo(Succeed())
			Expect(storePath).To(BeAnExistingFile())
		})
	})

	Context("when the store path doesn't exist", func() {
		It("does not fail", func() {
			Expect(Runner.WithStore("/tmp/not-really-a-thing").DeleteStore()).To(Succeed())
		})
	})
})

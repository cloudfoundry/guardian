package integration_test

import (
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/integration/runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Delete Store", func() {
	BeforeEach(func() {
		integration.SkipIfNonRoot(GrootfsTestUid)
	})

	It("empties the given store path", func() {
		Expect(Runner.InitStore(runner.InitSpec{})).To(Succeed())
		Expect(StorePath).To(BeAnExistingFile())

		storeContents, err := ioutil.ReadDir(StorePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(storeContents).ToNot(BeEmpty())

		Expect(Runner.DeleteStore()).To(Succeed())

		Expect(StorePath).ToNot(BeAnExistingFile())
	})

	Context("when given a path which does not look like a store", func() {
		JustBeforeEach(func() {
			Expect(os.MkdirAll(StorePath, 600)).To(Succeed())
			storeContents, err := ioutil.ReadDir(StorePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(storeContents).To(BeEmpty())
		})

		It("does not delete the directory", func() {
			Expect(Runner.DeleteStore()).To(MatchError(ContainSubstring("refusing to delete possibly corrupted store")))
			Expect(StorePath).To(BeAnExistingFile())
		})
	})

	Context("when the store path doesn't exist", func() {
		It("does not fail", func() {
			Expect(Runner.WithStore("/tmp/not-really-a-thing").DeleteStore()).To(Succeed())
		})
	})
})

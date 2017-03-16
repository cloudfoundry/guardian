package integration_test

import (
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/integration/runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Delete Store", func() {
	var (
		rootUID   int
		storePath string
		runner    runner.Runner
	)

	BeforeEach(func() {
		integration.SkipIfNonRoot(GrootfsTestUid)
		rootUID = 0
		var err error
		storePath, err = ioutil.TempDir(StorePath, "delete-store")
		Expect(err).NotTo(HaveOccurred())

		runner = Runner.WithStore(storePath)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath))
	})

	It("empties the given store path", func() {
		_, err := runner.Create(groot.CreateSpec{
			BaseImage: "docker:///busybox:1.26.2",
			ID:        "random-id",
			UIDMappings: []groot.IDMappingSpec{
				groot.IDMappingSpec{HostID: int(GrootUID), NamespaceID: 0, Size: 1},
				groot.IDMappingSpec{HostID: 100000, NamespaceID: 1, Size: 65000},
			},
			GIDMappings: []groot.IDMappingSpec{
				groot.IDMappingSpec{HostID: int(GrootGID), NamespaceID: 0, Size: 1},
				groot.IDMappingSpec{HostID: 100000, NamespaceID: 1, Size: 65000},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(storePath).To(BeAnExistingFile())
		storeContents, err := ioutil.ReadDir(storePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(storeContents).ToNot(BeEmpty())

		Expect(runner.DeleteStore()).To(Succeed())

		Expect(storePath).ToNot(BeAnExistingFile())
	})

	Context("when the store path is an empty folder", func() {
		JustBeforeEach(func() {
			storeContents, err := ioutil.ReadDir(storePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(storeContents).To(BeEmpty())
		})

		It("deletes the folder", func() {
			Expect(runner.DeleteStore()).To(Succeed())
			Expect(storePath).ToNot(BeAnExistingFile())
		})
	})

	Context("when the store path doesn't exist", func() {
		BeforeEach(func() {
			runner = runner.WithStore("/tmp/not-really-a-thing")
		})

		It("returns an error", func() {
			err := runner.DeleteStore()
			Expect(err).To(MatchError(ContainSubstring("store path doesn't exist")))
		})
	})
})

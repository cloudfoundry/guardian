package graph_test

import (
	"io/ioutil"
	"os"
	"path"

	"code.cloudfoundry.org/grootfs/graph"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Graph", func() {
	var (
		logger lager.Logger

		graphPath string

		grph *graph.Graph
	)

	BeforeEach(func() {
		var err error

		graphPath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		logger = lagertest.NewTestLogger("test-graph")
		grph = graph.NewGraph(graphPath)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(graphPath)).To(Succeed())
	})

	Describe("MakeBundle", func() {
		It("should return a bundle directory", func() {
			bundlePath, err := grph.MakeBundle(logger, "some-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(bundlePath).To(BeADirectory())
		})

		It("should keep the bundles in the same bundle directory", func() {
			Expect(grph.MakeBundle(logger, "some-id")).NotTo(BeEmpty())
			Expect(grph.MakeBundle(logger, "another-id")).NotTo(BeEmpty())

			bundles, err := ioutil.ReadDir(path.Join(graphPath, graph.BUNDLES_DIR_NAME))
			Expect(err).NotTo(HaveOccurred())
			Expect(len(bundles)).To(Equal(2))
		})

		Context("when calling it with two different ids", func() {
			It("should return two different bundle paths", func() {
				bundlePath, err := grph.MakeBundle(logger, "some-id")
				Expect(err).NotTo(HaveOccurred())

				anotherBundlePath, err := grph.MakeBundle(logger, "another-id")
				Expect(err).NotTo(HaveOccurred())

				Expect(bundlePath).NotTo(Equal(anotherBundlePath))
			})
		})

		Context("when using the same id twice", func() {
			It("should return an error", func() {
				_, err := grph.MakeBundle(logger, "some-id")
				Expect(err).NotTo(HaveOccurred())

				_, err = grph.MakeBundle(logger, "some-id")
				Expect(err).To(MatchError("bundle for id `some-id` already exists"))
			})
		})

		Context("when the graph path does not exist", func() {
			BeforeEach(func() {
				graphPath = "/non/existing/graph"
			})

			It("should return an error", func() {
				_, err := grph.MakeBundle(logger, "some-id")
				Expect(err).To(MatchError(ContainSubstring("making bundle path")))
			})
		})
	})
})

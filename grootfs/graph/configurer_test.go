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

var _ = Describe("Configurer", func() {
	var (
		graphPath string

		logger     lager.Logger
		configurer *graph.Configurer
	)

	BeforeEach(func() {
		tempDir, err := ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		graphPath = path.Join(tempDir, "graph")

		logger = lagertest.NewTestLogger("graph-configurer")
		configurer = graph.NewConfigurer()
	})

	AfterEach(func() {
		Expect(os.RemoveAll(path.Dir(graphPath))).To(Succeed())
	})

	Describe("Ensure", func() {
		It("should create the graph directory", func() {
			Expect(configurer.Ensure(logger, graphPath)).To(Succeed())

			Expect(graphPath).To(BeADirectory())
		})

		Context("when the base directory does not exist", func() {
			It("should return an error", func() {
				Expect(configurer.Ensure(logger, "/not/exist")).To(MatchError(ContainSubstring("making graph directory")))
			})
		})

		Context("when the graph already exists", func() {
			It("should succeed", func() {
				Expect(configurer.Ensure(logger, path.Dir(graphPath))).To(Succeed())
			})

			Context("and it's a regular file", func() {
				BeforeEach(func() {
					Expect(ioutil.WriteFile(graphPath, []byte("hello"), 0600)).To(Succeed())
				})

				It("should return an error", func() {
					Expect(configurer.Ensure(logger, graphPath)).To(MatchError(ContainSubstring("is not a directory")))
				})
			})
		})
	})
})

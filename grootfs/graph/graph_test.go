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
		imagePath string

		grph *graph.Graph
	)

	BeforeEach(func() {
		var err error

		graphPath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		imagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(path.Join(imagePath, "a_file"), []byte("hello-world"), 0600)).To(Succeed())
	})

	JustBeforeEach(func() {
		logger = lagertest.NewTestLogger("test-graph")
		grph = graph.NewGraph(graphPath)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(imagePath)).To(Succeed())
		Expect(os.RemoveAll(graphPath)).To(Succeed())
	})

	Describe("MakeBundle", func() {
		It("should return a bundle directory", func() {
			bundlePath, err := grph.MakeBundle(logger, imagePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(bundlePath).To(BeADirectory())
		})

		It("should have the image contents in the rootfs directory of the bundle", func() {
			bundlePath, err := grph.MakeBundle(logger, imagePath)
			Expect(err).NotTo(HaveOccurred())

			filePath := path.Join(bundlePath, "rootfs", "a_file")
			Expect(filePath).To(BeARegularFile())
			contents, err := ioutil.ReadFile(filePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("hello-world"))
		})

		Context("when the image path does not exist", func() {
			It("should return an error", func() {
				_, err := grph.MakeBundle(logger, "/does/not/exist")
				Expect(err).To(MatchError(ContainSubstring("image path `/does/not/exist` was not found")))
			})
		})

		Context("when the graph path does not exist", func() {
			BeforeEach(func() {
				graphPath = "/non/existing/graph"
			})

			It("should return an error", func() {
				_, err := grph.MakeBundle(logger, imagePath)
				Expect(err).To(MatchError(ContainSubstring("making bundle path")))
			})
		})

		Context("when rootfs is already configured in the bundle directory", func() {
			It("should return an error", func() {
				_, err := grph.MakeBundle(logger, imagePath)
				Expect(err).NotTo(HaveOccurred())

				_, err = grph.MakeBundle(logger, imagePath)
				Expect(err).To(MatchError(ContainSubstring("making bundle rootfs")))
			})
		})
	})
})

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
			bundlePath, err := grph.MakeBundle(logger, imagePath, "some-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(bundlePath).To(BeADirectory())
		})

		It("should keep the images in the same bundle directory", func() {
			Expect(grph.MakeBundle(logger, imagePath, "some-id")).NotTo(BeEmpty())
			Expect(grph.MakeBundle(logger, imagePath, "another-id")).NotTo(BeEmpty())

			bundles, err := ioutil.ReadDir(path.Join(graphPath, graph.BUNDLES_DIR_NAME))
			Expect(err).NotTo(HaveOccurred())
			Expect(len(bundles)).To(Equal(2))
		})

		It("should have the image contents in the rootfs directory of the bundle", func() {
			bundlePath, err := grph.MakeBundle(logger, imagePath, "some-id")
			Expect(err).NotTo(HaveOccurred())

			filePath := path.Join(bundlePath, "rootfs", "a_file")
			Expect(filePath).To(BeARegularFile())
			contents, err := ioutil.ReadFile(filePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("hello-world"))
		})

		Context("when calling it with two different ids", func() {
			It("should return two different bundle paths", func() {
				bundlePath, err := grph.MakeBundle(logger, imagePath, "some-id")
				Expect(err).NotTo(HaveOccurred())

				anotherBundlePath, err := grph.MakeBundle(logger, imagePath, "another-id")
				Expect(err).NotTo(HaveOccurred())

				Expect(bundlePath).NotTo(Equal(anotherBundlePath))
			})

			It("should isolate the rootfses when the same image is used", func() {
				bundlePath, err := grph.MakeBundle(logger, imagePath, "some-id")
				Expect(err).NotTo(HaveOccurred())

				anotherBundlePath, err := grph.MakeBundle(logger, imagePath, "another-id")
				Expect(err).NotTo(HaveOccurred())

				Expect(ioutil.WriteFile(path.Join(bundlePath, "rootfs", "bar"), []byte("hello-world"), 0644)).To(Succeed())
				Expect(path.Join(anotherBundlePath, "rootfs", "bar")).NotTo(BeARegularFile())
			})
		})

		Context("when the image path does not exist", func() {
			It("should return an error", func() {
				_, err := grph.MakeBundle(logger, "/does/not/exist", "some-id")
				Expect(err).To(MatchError(ContainSubstring("image path `/does/not/exist` was not found")))
			})
		})

		Context("when the graph path does not exist", func() {
			BeforeEach(func() {
				graphPath = "/non/existing/graph"
			})

			It("should return an error", func() {
				_, err := grph.MakeBundle(logger, imagePath, "some-id")
				Expect(err).To(MatchError(ContainSubstring("making bundle path")))
			})
		})

		Context("when using the same id twice", func() {
			It("should return an error", func() {
				_, err := grph.MakeBundle(logger, imagePath, "some-id")
				Expect(err).NotTo(HaveOccurred())

				_, err = grph.MakeBundle(logger, imagePath, "some-id")
				Expect(err).To(MatchError("bundle for id `some-id` already exists"))
			})
		})

		Context("when the image contains files that can only be read by root", func() {
			It("should return an error", func() {
				Expect(ioutil.WriteFile(path.Join(imagePath, "a-file"), []byte("hello-world"), 0000)).To(Succeed())

				_, err := grph.MakeBundle(logger, imagePath, "some-id")
				Expect(err).To(MatchError(ContainSubstring("copying the image in the bundle")))
			})
		})
	})
})

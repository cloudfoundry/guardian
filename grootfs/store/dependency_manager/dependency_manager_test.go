package dependency_manager_test

import (
	"os"
	"path"

	"code.cloudfoundry.org/grootfs/store/dependency_manager"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DependencyManager", func() {

	var (
		depsPath string
		manager  *dependency_manager.DependencyManager
	)

	BeforeEach(func() {
		var err error
		depsPath, err = os.MkdirTemp("", "dependencies")
		Expect(err).NotTo(HaveOccurred())

		manager = dependency_manager.NewDependencyManager(depsPath)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(depsPath)).To(Succeed())
	})

	Describe("Register", func() {
		It("register the dependencies for given image id", func() {
			imageID := "my-image"
			chainIDs := []string{"sha256:vol-1", "sha256:vol-2"}
			Expect(manager.Register(imageID, chainIDs)).To(Succeed())

			dependencies, err := manager.Dependencies(imageID)
			Expect(err).NotTo(HaveOccurred())
			Expect(dependencies).To(ConsistOf(chainIDs))
		})

		It("escapes the id", func() {
			imageID := "my/image"
			chainIDs := []string{"sha256:vol-1", "sha256:vol-2"}
			Expect(manager.Register(imageID, chainIDs)).To(Succeed())
			Expect(path.Join(depsPath, "my__image.json")).To(BeAnExistingFile())
		})

		Context("when the base path does not exist", func() {
			BeforeEach(func() {
				manager = dependency_manager.NewDependencyManager("/path/to/non/existent/dir")
			})

			It("return an error", func() {
				Expect(manager.Register("my-id", []string{"a-dep"})).To(
					MatchError(ContainSubstring("no such file or directory")),
				)
			})
		})
	})

	Describe("Deregister", func() {
		It("deregisters the dependencies for a given image", func() {
			imageID := "my-image"
			chainIDs := []string{"sha256:vol-1", "sha256:vol-2"}
			Expect(manager.Register(imageID, chainIDs)).To(Succeed())

			Expect(manager.Deregister(imageID)).To(Succeed())

			_, err := manager.Dependencies(imageID)
			Expect(err).To(MatchError(ContainSubstring("image `my-image` not found")))
		})

		It("escapes the id", func() {
			imageID := "my/image"
			chainIDs := []string{"sha256:vol-1", "sha256:vol-2"}
			Expect(manager.Register(imageID, chainIDs)).To(Succeed())

			Expect(manager.Deregister(imageID)).To(Succeed())
			Expect(path.Join(depsPath, "my__image.json")).ToNot(BeAnExistingFile())
		})

		Context("when the image does not exist", func() {
			It("returns an error", func() {
				Expect(manager.Deregister("my-image")).To(MatchError(ContainSubstring("no such file or directory")))
			})
		})
	})
})

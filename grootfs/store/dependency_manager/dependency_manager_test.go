package dependency_manager_test

import (
	"io/ioutil"
	"os"
	"path"

	"code.cloudfoundry.org/grootfs/store/dependency_manager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DependencyManager", func() {

	var (
		depsPath string
		manager  *dependency_manager.DependencyManager
	)

	BeforeEach(func() {
		var err error
		depsPath, err = ioutil.TempDir("", "dependencies")
		Expect(err).NotTo(HaveOccurred())

		manager = dependency_manager.NewDependencyManager(depsPath)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(depsPath)).To(Succeed())
	})

	Describe("Register", func() {
		It("register the dependencies for given bundle id", func() {
			bundleID := "my-bundle"
			chainIDs := []string{"sha256:vol-1", "sha256:vol-2"}
			Expect(manager.Register(bundleID, chainIDs)).To(Succeed())

			dependencies, err := manager.Dependencies(bundleID)
			Expect(err).NotTo(HaveOccurred())
			Expect(dependencies).To(ConsistOf(chainIDs))
		})

		It("escapes the id", func() {
			bundleID := "my/bundle"
			chainIDs := []string{"sha256:vol-1", "sha256:vol-2"}
			Expect(manager.Register(bundleID, chainIDs)).To(Succeed())
			Expect(path.Join(depsPath, "my__bundle.json")).To(BeAnExistingFile())
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
		It("deregisters the dependencies for a given bundle", func() {
			bundleID := "my-bundle"
			chainIDs := []string{"sha256:vol-1", "sha256:vol-2"}
			Expect(manager.Register(bundleID, chainIDs)).To(Succeed())

			Expect(manager.Deregister(bundleID)).To(Succeed())

			_, err := manager.Dependencies(bundleID)
			Expect(err).To(MatchError(ContainSubstring("bundle `my-bundle` not found")))
		})

		It("escapes the id", func() {
			bundleID := "my/bundle"
			chainIDs := []string{"sha256:vol-1", "sha256:vol-2"}
			Expect(manager.Register(bundleID, chainIDs)).To(Succeed())

			Expect(manager.Deregister(bundleID)).To(Succeed())
			Expect(path.Join(depsPath, "my__bundle.json")).ToNot(BeAnExistingFile())
		})

		Context("when the bundle does not exist", func() {
			It("returns an error", func() {
				Expect(manager.Deregister("my-bundle")).To(MatchError(ContainSubstring("no such file or directory")))
			})
		})
	})
})

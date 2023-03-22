package runcontainerd_test

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CgroupManager", func() {
	var (
		cgroupManager runcontainerd.CgroupManager
		runcRoot      string
	)

	BeforeEach(func() {
		runcRoot = tempDir("", "runc-root")
		cgroupManager = runcontainerd.NewCgroupManager(runcRoot, "garden")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(runcRoot)).To(Succeed())
	})

	Describe("SetUseMemoryHierarchy", func() {
		var (
			containerHandle  string
			useHierarchyPath string
			stateFilePath    string
			stateFileContent []byte
			setErr           error
			cgroupDir        string
		)

		BeforeEach(func() {
			containerHandle = "potato"

			cgroupDir = tempDir("", "memory-cgroup")

			useHierarchyPath = filepath.Join(cgroupDir, "memory.use_hierarchy")
			writeFile(useHierarchyPath, []byte("0"), os.ModePerm)

			stateFilePath = filepath.Join(runcRoot, "garden", containerHandle, "state.json")
			Expect(os.MkdirAll(filepath.Dir(stateFilePath), os.ModePerm)).To(Succeed())

			stateFileContent = marshal(map[string]interface{}{
				"cgroup_paths": map[string]interface{}{
					"memory": cgroupDir,
				},
			})
		})

		JustBeforeEach(func() {
			writeFile(stateFilePath, stateFileContent, os.ModePerm)
			setErr = cgroupManager.SetUseMemoryHierarchy(containerHandle)
		})

		AfterEach(func() {
			Expect(os.RemoveAll(cgroupDir)).To(Succeed())
		})

		It("sets memory.use_hierarchy to 1 for the specified container", func() {
			Expect(setErr).NotTo(HaveOccurred())
			Expect(ioutil.ReadFile(useHierarchyPath)).To(Equal([]byte("1")))
		})

		Context("when the state dir for the container doesn't exist", func() {
			BeforeEach(func() {
				containerHandle = "foo"
			})

			It("returns an error", func() {
				Expect(setErr).To(HaveOccurred())
			})
		})

		Context("when the container state file is malformed", func() {
			BeforeEach(func() {
				stateFileContent = nil
			})

			It("returns an error", func() {
				Expect(setErr).To(Equal(io.EOF))
			})
		})
	})
})

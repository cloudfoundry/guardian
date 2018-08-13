package runcontainerd_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CgroupManager", func() {
	var (
		cgroupManager runcontainerd.CgroupManager
		runcRoot      string
	)

	BeforeEach(func() {
		var err error
		runcRoot, err = ioutil.TempDir("", "runc-root")
		Expect(err).NotTo(HaveOccurred())

		cgroupManager = runcontainerd.NewCgroupManager(runcRoot, "garden")
	})

	Describe("SetUseMemoryHierarchy", func() {
		var (
			containerHandle           string
			memoryUseHierarchyFile    string
			containerStateFile        string
			containerStateFileContent []byte
			returnedErr               error
		)

		BeforeEach(func() {
			containerHandle = "potato"

			memoryDir, err := ioutil.TempDir("", "memory-cgroup")
			Expect(err).NotTo(HaveOccurred())

			memoryUseHierarchyFile = filepath.Join(memoryDir, "memory.use_hierarchy")
			Expect(ioutil.WriteFile(memoryUseHierarchyFile, []byte("0"), os.ModePerm)).To(Succeed())

			containerStateDir := filepath.Join(runcRoot, "garden", containerHandle)
			Expect(os.MkdirAll(containerStateDir, os.ModePerm)).To(Succeed())

			containerStateFile = filepath.Join(containerStateDir, "state.json")

			containerStateFileContent, err = json.Marshal(map[string]interface{}{
				"cgroup_paths": map[string]interface{}{
					"memory": memoryDir,
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			Expect(ioutil.WriteFile(containerStateFile, []byte(containerStateFileContent), os.ModePerm)).To(Succeed())

			returnedErr = cgroupManager.SetUseMemoryHierarchy(containerHandle)
		})

		It("sets memory.use_hierarchy to 1 for the specified container", func() {
			Expect(returnedErr).NotTo(HaveOccurred())
			Expect(ioutil.ReadFile(memoryUseHierarchyFile)).To(Equal([]byte("1")))
		})

		Context("when the state dir for the container doesn't exist", func() {
			BeforeEach(func() {
				containerHandle = "foo"
			})

			It("returns an error", func() {
				Expect(returnedErr).To(HaveOccurred())
			})
		})

		Context("when the container state file is malformed", func() {
			BeforeEach(func() {
				containerStateFileContent = []byte{}
			})

			It("returns an error", func() {
				Expect(returnedErr).To(MatchError(ContainSubstring("EOF")))
			})
		})
	})
})

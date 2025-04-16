package stopper_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	"code.cloudfoundry.org/guardian/rundmc/cgroups"
	"code.cloudfoundry.org/guardian/rundmc/stopper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resolver", func() {
	var (
		fakeStateDir         string
		notFoundRuntimeError map[string]string
	)

	BeforeEach(func() {
		var err error
		notFoundRuntimeError = map[string]string{
			"linux":   "no such file or directory",
			"windows": "The system cannot find the file specified.",
		}

		fakeStateDir, err = os.MkdirTemp("", "fakestate")
		Expect(err).NotTo(HaveOccurred())

		Expect(os.MkdirAll(filepath.Join(fakeStateDir, "some-handle"), 0700)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(fakeStateDir)).To(Succeed())
	})

	Context("with valid state.json", func() {
		BeforeEach(func() {
			stateJson, err := os.Create(filepath.Join(fakeStateDir, "some-handle", "state.json"))
			Expect(err).NotTo(HaveOccurred())

			if cgroups.IsCgroup2UnifiedMode() {
				Expect(json.NewEncoder(stateJson).Encode(map[string]interface{}{
					"cgroup_paths": map[string]string{
						"": "i-am-the-devices-cgroup-path",
					},
				})).To(Succeed())
			} else {
				Expect(json.NewEncoder(stateJson).Encode(map[string]interface{}{
					"cgroup_paths": map[string]string{
						"devices": "i-am-the-devices-cgroup-path",
					},
				})).To(Succeed())
			}
			Expect(stateJson.Close()).To(Succeed())
		})

		It("resolves the cgroup of a process by reading state.json", func() {
			path, err := stopper.NewRuncStateCgroupPathResolver(fakeStateDir).Resolve("some-handle", "devices")
			Expect(err).NotTo(HaveOccurred())
			Expect(path).To(Equal("i-am-the-devices-cgroup-path"))
		})
	})

	Context("with invalid state.json", func() {
		BeforeEach(func() {
			stateJson, err := os.Create(filepath.Join(fakeStateDir, "some-handle", "state.json"))
			Expect(err).NotTo(HaveOccurred())

			stateJson.WriteString("k!tK@T")
			Expect(stateJson.Close()).To(Succeed())
		})

		It("resolves the cgroup of a process by reading state.json", func() {
			_, err := stopper.NewRuncStateCgroupPathResolver(fakeStateDir).Resolve("some-handle", "devices")
			Expect(err).To(MatchError(ContainSubstring("invalid character")))
		})
	})

	It("returns an error if the state.json doesn't exist", func() {
		_, err := stopper.NewRuncStateCgroupPathResolver(fakeStateDir).Resolve("some-handle", "devices")
		Expect(err).To(MatchError(ContainSubstring(notFoundRuntimeError[runtime.GOOS])))
	})
})

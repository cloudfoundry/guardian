package goci_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"

	"code.cloudfoundry.org/guardian/rundmc/goci"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Bundle Serialization", func() {

	var (
		tmp         string
		bndle       goci.Bndl
		bundleSaver *goci.BundleSaver
	)

	BeforeEach(func() {
		var err error
		tmp, err = ioutil.TempDir("", "gocitest")
		Expect(err).NotTo(HaveOccurred())

		bundleSaver = &goci.BundleSaver{}

		bndle = goci.Bndl{
			Spec: specs.Spec{
				Version: "abcd",
			},
		}

		Expect(bundleSaver.Save(bndle, tmp)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmp)).To(Succeed())
	})

	Describe("Saving", func() {
		It("serializes the spec to config.json", func() {
			var configJson map[string]interface{}
			config := mustOpen(filepath.Join(tmp, "config.json"))
			defer config.Close()
			Expect(json.NewDecoder(config).Decode(&configJson)).To(Succeed())
			Expect(configJson).To(HaveKeyWithValue("ociVersion", Equal("abcd")))
		})

		It("ensures that the runc spec is only readable by its owner", func() {
			if runtime.GOOS == "windows" {
				Skip("not supported on Windows")
			}

			info, err := os.Stat(filepath.Join(tmp, "config.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))
		})

		Context("when saving fails", func() {
			It("returns an error", func() {
				err := bundleSaver.Save(bndle, "non-existent-dir")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("Failed to save bundle")))
				pathNotFoundErrorStr := "no such file or directory"
				if runtime.GOOS == "windows" {
					pathNotFoundErrorStr = "The system cannot find the path specified."
				}
				Expect(err).To(MatchError(ContainSubstring(pathNotFoundErrorStr)))
			})
		})
	})

	Describe("Loading", func() {
		Context("when config.json exist", func() {
			It("loads the bundle from config.json", func() {
				bundleLoader := &goci.BndlLoader{}
				loadedBundle, _ := bundleLoader.Load(tmp)
				Expect(loadedBundle).To(Equal(bndle))
			})
		})

		Context("when config.json does not exist", func() {
			It("returns an error", func() {
				Expect(os.Remove(path.Join(tmp, "config.json"))).To(Succeed())
				bundleLoader := &goci.BndlLoader{}
				_, err := bundleLoader.Load(tmp)
				Expect(err).To(MatchError(ContainSubstring("Failed to load bundle")))
				fileNotFoundErrorStr := "no such file or directory"
				if runtime.GOOS == "windows" {
					fileNotFoundErrorStr = "The system cannot find the file specified."
				}
				Expect(err).To(MatchError(ContainSubstring(fileNotFoundErrorStr)))
			})
		})

		Context("when config.json is not valid bundle", func() {
			BeforeEach(func() {
				ioutil.WriteFile(path.Join(tmp, "config.json"), []byte("appended-nonsense"), 0755)
			})

			It("returns an error", func() {
				bundleLoader := &goci.BndlLoader{}
				_, err := bundleLoader.Load(tmp)
				Expect(err).To(MatchError(ContainSubstring("Failed to load bundle")))
				Expect(err).To(MatchError(ContainSubstring("invalid character")))
			})
		})
	})
})

func mustOpen(path string) *os.File {
	r, err := os.Open(path)
	Expect(err).NotTo(HaveOccurred())
	return r
}

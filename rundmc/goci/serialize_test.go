package goci_test

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path"

	"code.cloudfoundry.org/guardian/rundmc/goci"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Bundle Serialization", func() {

	var (
		tmp   string
		bndle goci.Bndl
	)

	BeforeEach(func() {
		var err error
		tmp, err = ioutil.TempDir("", "gocitest")
		Expect(err).NotTo(HaveOccurred())

		bndle = goci.Bndl{
			Spec: specs.Spec{
				Version: "abcd",
			},
		}

		Expect(bndle.Save(tmp)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmp)).To(Succeed())
	})

	Describe("Saving", func() {
		It("serializes the spec to spec.json", func() {
			var configJson map[string]interface{}
			Expect(json.NewDecoder(mustOpen(path.Join(tmp, "config.json"))).Decode(&configJson)).To(Succeed())

			Expect(configJson).To(HaveKeyWithValue("ociVersion", Equal("abcd")))
		})

		Context("when saving fails", func() {
			It("returns an error", func() {
				err := bndle.Save("non-existent-dir")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("Failed to save bundle")))
				Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
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
				Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
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

func mustOpen(path string) io.Reader {
	r, err := os.Open(path)
	Expect(err).NotTo(HaveOccurred())

	return r
}

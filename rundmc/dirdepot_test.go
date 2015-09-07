package rundmc_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/cf-guardian/specs"
	"github.com/cloudfoundry-incubator/guardian/rundmc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Depot", func() {
	Describe("create", func() {
		var tmpDir string
		BeforeEach(func() {
			var err error

			tmpDir, err = ioutil.TempDir("", "depot-test")
			Expect(err).NotTo(HaveOccurred())

			depot := rundmc.DirectoryDepot{
				Dir: tmpDir,
			}

			Expect(depot.Create("aardvaark")).To(Succeed())
		})

		It("should create a directory", func() {
			Expect(filepath.Join(tmpDir, "aardvaark")).To(BeADirectory())
		})

		Describe("the container directory", func() {
			It("should contain a config.json", func() {
				Expect(filepath.Join(tmpDir, "aardvaark", "config.json")).To(BeARegularFile())
			})

			It("should have a config.json with a process which echos 'Pid 1 Running'", func() {
				file, err := os.Open(filepath.Join(tmpDir, "aardvaark", "config.json"))
				Expect(err).NotTo(HaveOccurred())

				var target specs.Spec
				Expect(json.NewDecoder(file).Decode(&target)).To(Succeed())

				Expect(target.Process).To(Equal(specs.Process{
					Args: []string{
						"/bin/echo", "Pid 1 Running",
					},
				}))
			})

			It("should have a config.json which specifies the spec version", func() {
				file, err := os.Open(filepath.Join(tmpDir, "aardvaark", "config.json"))
				Expect(err).NotTo(HaveOccurred())

				var target specs.Spec
				Expect(json.NewDecoder(file).Decode(&target)).To(Succeed())

				Expect(target.Version).To(Equal("pre-draft"))
			})
		})
	})
})

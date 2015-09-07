package rundmc_test

import (
	"io/ioutil"
	"path/filepath"

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
		})
	})
})

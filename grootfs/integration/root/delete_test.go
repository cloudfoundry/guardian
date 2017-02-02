package root_test

import (
	"fmt"
	"io/ioutil"
	"path"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Delete", func() {
	var image groot.Image

	BeforeEach(func() {
		sourceImagePath, err := ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(path.Join(sourceImagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())
		baseImageFile := integration.CreateBaseImageTar(sourceImagePath)
		image, err = Runner.Create(groot.CreateSpec{
			BaseImage: baseImageFile.Name(),
			ID:        "random-id",
		})
		Expect(err).ToNot(HaveOccurred())
	})

	Context("when trying to delete a image from a different user", func() {
		It("doesn't return an error but logs a warning", func() {
			storePath, err := ioutil.TempDir(StorePath, "")
			Expect(err).NotTo(HaveOccurred())

			outBuffer := gbytes.NewBuffer()
			err = Runner.RunningAsUser(GrootUID, GrootGID).WithStore(storePath).WithLogLevel(lager.DEBUG).WithStdout(outBuffer).
				Delete(image.Path)
			Expect(err).NotTo(HaveOccurred())

			Eventually(outBuffer).Should(gbytes.Say(fmt.Sprintf("path `%s` is outside store path", image.Path)))
			Expect(image.Path).To(BeADirectory())
		})
	})
})

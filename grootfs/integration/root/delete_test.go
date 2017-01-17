package root_test

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"syscall"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Delete", func() {
	var image groot.Image

	BeforeEach(func() {
		sourceImagePath, err := ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(path.Join(sourceImagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())
		baseImageFile := integration.CreateBaseImageTar(sourceImagePath)
		image = integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImageFile.Name(), "random-id", 0)
	})

	Context("when trying to delete a image from a different user", func() {
		It("doesn't return an error but logs a warning", func() {
			deleteCmd := exec.Command(
				GrootFSBin,
				"--log-level", "debug",
				"--store", StorePath,
				"--drax-bin", DraxBin,
				"delete",
				image.Path,
			)
			deleteCmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: GrootUID,
					Gid: GrootGID,
				},
			}

			session, err := gexec.Start(deleteCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("path `%s` is outside store path", image.Path)))
			Expect(image.Path).To(BeADirectory())
		})
	})
})

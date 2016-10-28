package root_test

import (
	"io/ioutil"
	"os/exec"
	"path"
	"syscall"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Delete", func() {
	var (
		imagePath string
		bundle    groot.Bundle
	)

	BeforeEach(func() {
		var err error
		imagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(path.Join(imagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())
		bundle = integration.CreateBundle(GrootFSBin, StorePath, DraxBin, imagePath, "random-id", 0)
	})

	Context("when trying to delete a bundle from a different user", func() {
		It("returns an error", func() {
			deleteCmd := exec.Command(
				GrootFSBin,
				"--log-level", "debug",
				"--store", StorePath,
				"--drax-bin", DraxBin,
				"delete",
				bundle.Path,
			)
			deleteCmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: GrootUID,
					Gid: GrootGID,
				},
			}

			session, err := gexec.Start(deleteCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(bundle.Path).To(BeADirectory())
		})
	})
})

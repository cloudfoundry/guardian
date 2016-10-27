package integration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"syscall"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Newgidmap", func() {
	Context("when the user is maximus", func() {
		var sourcePath string

		BeforeEach(func() {
			var err error
			sourcePath, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(sourcePath)).To(Succeed())
		})

		shouldMapFileGroupToGID := func(filePath string, gidMapping string) {
			statCmd := exec.Command(NamespaceWrapperBin, "stat", "-c", "%g", filePath)
			statCmd.SysProcAttr = &syscall.SysProcAttr{
				Cloneflags: syscall.CLONE_NEWUSER,
			}
			statCmd.Stderr = GinkgoWriter

			buffer := gbytes.NewBuffer()
			statCmd.Stdout = buffer

			pipeR, pipeW, err := os.Pipe()
			Expect(err).NotTo(HaveOccurred())
			statCmd.ExtraFiles = []*os.File{pipeR}
			Expect(statCmd.Start()).To(Succeed())

			idmapperCmd := exec.Command(NewgidmapBin, fmt.Sprintf("%d", statCmd.Process.Pid))
			idmapperCmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: MaximusID,
					Gid: MaximusID,
				},
			}
			sess, err := gexec.Start(idmapperCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))

			_, err = pipeW.Write([]byte{0})
			Expect(err).NotTo(HaveOccurred())
			Expect(statCmd.Wait()).To(Succeed())
			Eventually(buffer).Should(gbytes.Say(gidMapping))
		}

		It("correctly maps maximus user id", func() {
			maximusFilePath := path.Join(sourcePath, "maximus")
			Expect(ioutil.WriteFile(maximusFilePath, []byte("hello-world"), 0644)).To(Succeed())
			Expect(os.Chown(maximusFilePath, int(MaximusID), int(MaximusID))).To(Succeed())

			shouldMapFileGroupToGID(maximusFilePath, fmt.Sprintf("%d", RootID))
		})

		It("correctly maps root user id", func() {
			rootFilePath := path.Join(sourcePath, "root")
			Expect(ioutil.WriteFile(rootFilePath, []byte("hello-world"), 0644)).To(Succeed())
			Expect(os.Chown(rootFilePath, int(RootID), int(RootID))).To(Succeed())

			shouldMapFileGroupToGID(rootFilePath, fmt.Sprintf("%d", NobodyID))
		})

		It("correctly maps user 102 id", func() {
			user102FilePath := path.Join(sourcePath, "102")
			Expect(ioutil.WriteFile(user102FilePath, []byte("hello-world"), 0644)).To(Succeed())
			Expect(os.Chown(user102FilePath, 102, 102)).To(Succeed())

			shouldMapFileGroupToGID(user102FilePath, fmt.Sprintf("%d", 102))
		})
	})

	Context("when the user is not maximus", func() {
		It("dies a horrible death", func() {
			idmapperCmd := exec.Command(NewgidmapBin, "1234")
			idmapperCmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: 1000,
					Gid: 1000,
				},
			}

			sess, err := gexec.Start(idmapperCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).ShouldNot(gexec.Exit(0))
			Eventually(sess.Err).Should(
				gbytes.Say(fmt.Sprintf("you can only run this as user %d", MaximusID)),
			)
		})
	})

	Context("when the process does not exist", func() {
		It("returns an error", func() {
			idmapperCmd := exec.Command(NewgidmapBin, "123412341234")
			idmapperCmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: MaximusID,
					Gid: MaximusID,
				},
			}

			sess, err := gexec.Start(idmapperCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).ShouldNot(gexec.Exit(0))
			Eventually(sess.Err).Should(
				gbytes.Say(fmt.Sprintf("no such file or directory")),
			)
		})
	})

	Context("when the PID is invalid", func() {
		It("returns an error", func() {
			idmapperCmd := exec.Command(NewgidmapBin, "120/../1")
			idmapperCmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: MaximusID,
					Gid: MaximusID,
				},
			}

			sess, err := gexec.Start(idmapperCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).ShouldNot(gexec.Exit(0))
			Eventually(sess.Err).Should(
				gbytes.Say(fmt.Sprintf("invalid syntax")),
			)
		})
	})
})

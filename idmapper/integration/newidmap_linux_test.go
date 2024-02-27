package integration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("newgidmap", func() {
	testIDMapperBins(func() string { return NewgidmapBin }, "%g")
})

var _ = Describe("newuidmap", func() {
	testIDMapperBins(func() string { return NewuidmapBin }, "%u")
})

func testIDMapperBins(bin func() string, statFmt string) {
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

		shouldMapFileGroupToID := func(filePath string, idMapping string) {
			statCmd := exec.Command(NamespaceWrapperBin, "stat", "-c", statFmt, filePath)
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

			idmapperCmd := exec.Command(bin(), fmt.Sprintf("%d", statCmd.Process.Pid))
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
			Eventually(buffer).Should(gbytes.Say(idMapping))
		}

		itCorrectlyMapsID := func(id, expectedId uint32) {
			filePath := path.Join(sourcePath, "foo")
			Expect(ioutil.WriteFile(filePath, []byte("hello-world"), 0644)).To(Succeed())
			Expect(os.Chown(filePath, int(id), int(id))).To(Succeed())

			shouldMapFileGroupToID(filePath, fmt.Sprintf("%d", expectedId))
		}

		It("correctly maps maximus", func() {
			itCorrectlyMapsID(MaximusID, RootID)
		})

		It("correctly maps root", func() {
			itCorrectlyMapsID(RootID, overflowID)
		})

		It("does not map ids smaller than 65536", func() {
			itCorrectlyMapsID(1000, overflowID)
		})

		It("correctly maps 65640", func() {
			itCorrectlyMapsID(65535+105, 105)
		})
	})

	Context("when neither the uid nor gid of the process are maximus", func() {
		It("dies a horrible death", func() {
			idmapperCmd := exec.Command(bin(), "1234")
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
			idmapperCmd := exec.Command(bin(), "123412341234")
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
				gbytes.Say("no such file or directory"),
			)
		})
	})

	Context("when the PID is invalid", func() {
		It("returns an error", func() {
			idmapperCmd := exec.Command(bin(), "120/../1")
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
				gbytes.Say("invalid syntax"),
			)
		})
	})
}

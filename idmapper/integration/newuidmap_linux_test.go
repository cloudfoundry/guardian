package integration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Newuidmap", func() {

	var (
		sourcePath    string
		grootFilePath string
		rootFilePath  string
	)

	BeforeEach(func() {
		var err error
		sourcePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		Expect(os.MkdirAll("/var/vcap/jobs/grootfs", 755)).To(Succeed())
		Expect(ioutil.WriteFile("/var/vcap/jobs/grootfs/subuid", []byte("groot:100000:65000"), 0644)).To(Succeed())

		grootFilePath = path.Join(sourcePath, "foo")
		Expect(ioutil.WriteFile(grootFilePath, []byte("hello-world"), 0644)).To(Succeed())
		Expect(os.Chown(grootFilePath, int(GrootUID), int(GrootGID))).To(Succeed())

		rootFilePath = path.Join(sourcePath, "bar")
		Expect(ioutil.WriteFile(rootFilePath, []byte("hello-world"), 0644)).To(Succeed())
	})

	shouldMapFileOwnerToUID := func(filePath string, uidMapping string) {
		statCmd := exec.Command(NamespaceWrapperBin, "stat", "-c", "%u", filePath)
		statCmd.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWUSER,
		}

		buffer := gbytes.NewBuffer()
		statCmd.Stdout = buffer
		pipeR, pipeW, err := os.Pipe()
		Expect(err).NotTo(HaveOccurred())
		statCmd.ExtraFiles = []*os.File{pipeR}
		Expect(statCmd.Start()).To(Succeed())

		args := fmt.Sprintf("%d %d %d %d %d %d %d", statCmd.Process.Pid, 0, GrootUID, 1, 1, 100000, 65000)
		newuidmapCmd := exec.Command(NewuidmapBin, strings.Split(args, " ")...)
		newuidmapCmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: GrootUID,
				Gid: GrootGID,
			},
		}
		newuidmapCmd.Stderr = GinkgoWriter
		newuidmapCmd.Stdout = GinkgoWriter
		Expect(newuidmapCmd.Run()).To(Succeed())
		_, err = pipeW.Write([]byte{0})
		Expect(err).NotTo(HaveOccurred())

		Expect(statCmd.Wait()).To(Succeed())
		Eventually(buffer).Should(gbytes.Say(uidMapping))
	}

	It("correctly maps groot user id", func() {
		shouldMapFileOwnerToUID(grootFilePath, fmt.Sprintf("%d", GrootUID+99999))
	})

	It("correctly maps root user id", func() {
		shouldMapFileOwnerToUID(rootFilePath, fmt.Sprintf("%d", GrootUID))
	})

	Context("validating subuid range", func() {
		var (
			statCmd *exec.Cmd
			pipeW   *os.File
		)

		BeforeEach(func() {
			var err error

			statCmd = exec.Command(NamespaceWrapperBin, "stat", "-c", "%u", grootFilePath)
			statCmd.SysProcAttr = &syscall.SysProcAttr{
				Cloneflags: syscall.CLONE_NEWUSER,
			}

			var pipeR *os.File
			pipeR, pipeW, err = os.Pipe()
			Expect(err).NotTo(HaveOccurred())
			statCmd.ExtraFiles = []*os.File{pipeR}

			_, err = gexec.Start(statCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the range is empty", func() {
			It("fails", func() {
				args := fmt.Sprintf("%d %d %d %d %d %d %d", statCmd.Process.Pid, 0, GrootUID, 0, 0, 100000, 65000)
				newuidmapCmd := exec.Command(NewuidmapBin, strings.Split(args, " ")...)

				session, err := gexec.Start(newuidmapCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1))
				_, err = pipeW.Write([]byte{0})

				Expect(err).ToNot(HaveOccurred())
				Expect(session.Err).To(gbytes.Say("mapping 0:1000:0 invalid: size can't be zero"))
			})
		})

		Context("when the range isn't allowed in the custom subuid file", func() {
			It("fails", func() {
				args := fmt.Sprintf("%d %d %d %d %d %d %d", statCmd.Process.Pid, 0, GrootUID, 1, 1, 1000, 1000000)
				newuidmapCmd := exec.Command(NewuidmapBin, strings.Split(args, " ")...)

				session, err := gexec.Start(newuidmapCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1))
				_, err = pipeW.Write([]byte{0})

				Expect(err).ToNot(HaveOccurred())
				Expect(session.Err).To(gbytes.Say("mapping 0:1000:1 invalid: range is not allowed"))
			})
		})
	})
})

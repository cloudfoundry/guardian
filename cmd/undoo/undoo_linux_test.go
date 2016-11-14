package main_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/kr/logfmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Undoo", func() {
	var (
		logFile, logDir string
	)

	BeforeEach(func() {
		var err error
		logDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		logFile = filepath.Join(logDir, "undoo.log")
	})

	It("blows up if -log-file is not specified", func() {
		cmd := exec.Command(undooBinPath)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session).Should(gexec.Exit(1))
	})

	Context("when given a logfile that can't be created", func() {
		var session *gexec.Session

		BeforeEach(func() {
			var err error
			cmd := exec.Command(undooBinPath, "-log-file", "/non/existent")
			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
		})

		It("exits with code 2", func() {
			Eventually(session).Should(gexec.Exit(2))
		})
	})

	It("creates a new mount namespace", func() {
		parentNsCmd := exec.Command("readlink", "/proc/self/ns/mnt")
		parentNsBytes, err := parentNsCmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred())

		cmd := exec.Command(undooBinPath, "-log-file", logFile, "mountsRoot", "keep-id", "readlink", "/proc/self/ns/mnt")
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Eventually(session).Should(gexec.Exit(0))
		Expect(err).NotTo(HaveOccurred())

		Consistently(session).ShouldNot(Equal(parentNsBytes))
	})

	Context("when the cmd is invalid", func() {
		It("logs that it failed to start the cmd to the end of the log file", func() {
			cmd := exec.Command(undooBinPath, "-log-file", logFile, "mountsRoot", "keep-id", "scoobydoo")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(4))

			parsedLogLine := struct{ Msg string }{}
			logfmt.Unmarshal([]byte(lastLogLine(logFile)), &parsedLogLine)
			Expect(parsedLogLine.Msg).To(ContainSubstring("executable file not found in $PATH"))
		})
	})

	Context("when the cmd to call return an error", func() {
		It("ensures the cmd's error is the last line in the log file", func() {
			cmd := exec.Command(undooBinPath, "-log-file", logFile, "mountsRoot", "keep-id", "/bin/bash", "-c", fmt.Sprintf("ls scoobydoo 2>>%s", logFile))
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(2))

			Expect(lastLogLine(logFile)).To(ContainSubstring("No such file or directory"))
		})
	})

	Context("when there are mounts under depot path in the parent mount namespace", func() {
		var tmpDir, depotPath, mnt1, mnt2 string

		BeforeEach(func() {
			var err error
			tmpDir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
			depotPath = filepath.Join(tmpDir, "aufs")
			Expect(os.MkdirAll(depotPath, 0644)).To(Succeed())

			Expect(syscall.Mount(depotPath, depotPath, "", syscall.MS_BIND, "")).To(Succeed())

			mnt1 = filepath.Join(depotPath, "mnt1")
			Expect(os.MkdirAll(mnt1, 0644)).To(Succeed())
			Expect(syscall.Mount("tmpfs", mnt1, "tmpfs", 0, "")).To(Succeed())

			mnt2 = filepath.Join(depotPath, "mnt2")
			Expect(os.MkdirAll(mnt2, 0644)).To(Succeed())
			Expect(syscall.Mount("tmpfs", mnt2, "tmpfs", 0, "")).To(Succeed())
		})

		AfterEach(func() {
			Expect(syscall.Unmount(mnt1, 0)).To(Succeed())
			Expect(syscall.Unmount(mnt2, 0)).To(Succeed())
			Expect(syscall.Unmount(depotPath, 0)).To(Succeed())
			Expect(os.RemoveAll(tmpDir)).To(Succeed())
		})

		It("unmounts all unneeded mounts from the child mount namespace", func() {
			cmd := exec.Command(undooBinPath, "-log-file", logFile, depotPath, "mnt2", "/bin/bash", "-c", "cat /proc/mounts > mounts")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			Expect(readFile("mounts")).NotTo(ContainSubstring("mnt1"))
			Expect(readFile("mounts")).To(ContainSubstring("mnt2"))

			mountsBytes, err := exec.Command("cat", "/proc/self/mounts").CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			mounts := string(mountsBytes)

			Expect(mounts).To(ContainSubstring("mnt1"))
			Expect(mounts).To(ContainSubstring("mnt2"))
		})
	})
})

func readFile(fileName string) string {
	contents, err := ioutil.ReadFile(fileName)
	Expect(err).NotTo(HaveOccurred())
	return strings.TrimSpace(string(contents))
}

func lastLogLine(logFile string) string {
	contents := readFile(logFile)
	lines := strings.Split(contents, "\n")
	return lines[len(lines)-1]
}

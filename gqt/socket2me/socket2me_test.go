package socket2me_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Socket2Me", func() {
	var (
		tempDir    string
		socketPath string
	)

	BeforeEach(func() {
		var err error
		tempDir, err = ioutil.TempDir("", "socket2me-tests")
		Expect(err).NotTo(HaveOccurred())
		socketPath = filepath.Join(tempDir, "somefile.sock")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tempDir)).To(Succeed())
	})

	It("creates the socket file", func() {
		runSocketToMe(socketPath, &bytes.Buffer{}, "/bin/true")
		file, err := os.Stat(socketPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(file.Mode() & os.ModeSocket).To(Equal(os.ModeSocket))
	})

	It("execs the passed command", func() {
		var stdout bytes.Buffer
		proc := runSocketToMe(socketPath, &stdout, "/bin/bash", "-c", "echo $$")
		Expect(stdout.String()).To(Equal(fmt.Sprintf("%d\n", proc.Pid)))
	})

	itSetsEnvVars := func() {
		It("sets socket FD env var for command", func() {
			var stdout bytes.Buffer
			runSocketToMe(socketPath, &stdout, "/bin/bash", "-c", "readlink /proc/self/fd/${SOCKET2ME_FD}")
			Expect(stdout.String()).To(ContainSubstring("socket:["))
		})

		It("preserves the original environment", func() {
			var stdout bytes.Buffer
			runSocketToMe(socketPath, &stdout, "/usr/bin/env")
			for _, entry := range os.Environ() {
				Expect(stdout.String()).To(ContainSubstring(entry))
			}
		})
	}

	itSetsEnvVars()

	It("runs the given command as the passed user and group", func() {
		var stdout bytes.Buffer
		runSocketToMe(socketPath, &stdout, "/usr/bin/id")
		Expect(stdout.String()).To(Equal("uid=2000 gid=3000\n"))
	})

	Context("when the socket file already exists", func() {
		BeforeEach(func() {
			fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(syscall.Bind(fd, &syscall.SockaddrUnix{Name: socketPath})).To(Succeed())
		})

		It("still succeeds", func() {
			runSocketToMe(socketPath, &bytes.Buffer{}, "/bin/true")
			file, err := os.Stat(socketPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(file.Mode() & os.ModeSocket).To(Equal(os.ModeSocket))
		})

		itSetsEnvVars()
	})

	Context("When the socket path is not passed", func() {
		It("fails", func() {
			socketToMeCmd := exec.Command(socket2MeBinPath)
			socketToMeCmd.Stdout = GinkgoWriter
			socketToMeCmd.Stderr = GinkgoWriter
			Expect(socketToMeCmd.Run()).NotTo(Succeed())
		})
	})
})

func runSocketToMe(socketPath string, stdout io.Writer, cmdArgv ...string) *os.Process {
	args := []string{"--uid=2000", "--gid=3000"}
	if socketPath != "" {
		args = append(args, "--socket-path", socketPath)
	}
	args = append(args, cmdArgv...)

	socketToMeCmd := exec.Command(socket2MeBinPath, args...)
	socketToMeCmd.Stdout = io.MultiWriter(stdout, GinkgoWriter)
	socketToMeCmd.Stderr = GinkgoWriter
	Expect(socketToMeCmd.Run()).To(Succeed())
	return socketToMeCmd.Process
}

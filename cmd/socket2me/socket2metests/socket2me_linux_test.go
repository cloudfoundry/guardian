package socket2metests_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	It("execs commands that appear to have flags of their own", func() {
		var stdout bytes.Buffer
		runSocketToMe(socketPath, &stdout, "/bin/echo", "--foo", "bar")
		Expect(stdout.String()).To(Equal("--foo bar\n"))
	})

	itChownsAndChmodsTheSocket := func() {
		It("chowns the socket to the configured uid and gid with mode 600", func() {
			runSocketToMe(socketPath, &bytes.Buffer{}, "/bin/true")
			socketPathInfo, err := os.Stat(socketPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(socketPathInfo.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(4000)))
			Expect(socketPathInfo.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(5000)))
			Expect(socketPathInfo.Mode() & os.ModePerm).To(Equal(os.FileMode(0600)))
		})
	}

	itChownsAndChmodsTheSocket()

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

	It("runs the given command as the passed user and group, with no supplementary groups", func() {
		var stdout bytes.Buffer
		runSocketToMe(socketPath, &stdout, "/usr/bin/id")
		outputString := stdout.String()
		if strings.Contains(outputString, "groups") {
			Expect(outputString).To(Equal("uid=2000 gid=3000 groups=3000\n"))
		} else {
			Expect(outputString).To(Equal("uid=2000 gid=3000\n"))
		}
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
		itChownsAndChmodsTheSocket()
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
	args := []string{"--uid=2000", "--gid=3000", "--socket-uid=4000", "--socket-gid=5000"}
	if socketPath != "" {
		args = append(args, "--socket-path", socketPath)
	}
	args = append(args, cmdArgv...)

	socketToMeCmd := exec.Command(socket2MeBinPath, args...)
	socketToMeCmd.Stdout = io.MultiWriter(stdout, GinkgoWriter)
	socketToMeCmd.Stderr = GinkgoWriter
	socketToMeCmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid:    0,
			Gid:    0,
			Groups: []uint32{10001},
		},
	}
	Expect(socketToMeCmd.Run()).To(Succeed())
	return socketToMeCmd.Process
}

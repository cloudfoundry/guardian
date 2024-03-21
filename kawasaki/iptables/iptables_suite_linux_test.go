//go:build linux

package iptables_test

import (
	"bytes"
	"io"
	"os/exec"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func run(cmd *exec.Cmd) (int, string) {
	var buff bytes.Buffer
	cmd.Stdout = io.MultiWriter(&buff, GinkgoWriter)
	cmd.Stderr = GinkgoWriter
	Expect(cmd.Start()).To(Succeed())
	err := cmd.Wait()
	if err == nil {
		return 0, buff.String()
	}
	exitErr := err.(*exec.ExitError)
	return exitErr.Sys().(syscall.WaitStatus).ExitStatus(), buff.String()
}

func runForStdout(cmd *exec.Cmd) string {
	exitCode, stdout := run(cmd)
	Expect(exitCode).To(Equal(0))
	return stdout
}

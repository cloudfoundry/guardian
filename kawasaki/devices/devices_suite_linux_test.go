//go:build linux

package devices_test

import (
	"bytes"
	"io"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func startCommand(arg0 string, argv ...string) (*exec.Cmd, *bytes.Buffer) {
	cmd := exec.Command(arg0, argv...)
	stdout := new(bytes.Buffer)
	cmd.Stdout = io.MultiWriter(stdout, GinkgoWriter)
	cmd.Stderr = GinkgoWriter
	ExpectWithOffset(1, cmd.Start()).To(Succeed())
	return cmd, stdout
}

func runCommand(arg0 string, argv ...string) string {
	cmd, stdout := startCommand(arg0, argv...)
	ExpectWithOffset(1, cmd.Wait()).To(Succeed())
	return stdout.String()
}

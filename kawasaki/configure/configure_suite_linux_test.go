//go:build linux

package configure_test

import (
	"bytes"
	"io"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func runCommand(arg0 string, argv ...string) string {
	var stdout bytes.Buffer
	cmd := exec.Command(arg0, argv...)
	cmd.Stdout = io.MultiWriter(&stdout, GinkgoWriter)
	cmd.Stderr = GinkgoWriter
	ExpectWithOffset(1, cmd.Run()).To(Succeed())
	return stdout.String()
}

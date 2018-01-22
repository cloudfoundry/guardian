package configure_test

import (
	"bytes"
	"io"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestConfigure(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Configure Suite")
}

func runCommand(arg0 string, argv ...string) string {
	var stdout bytes.Buffer
	cmd := exec.Command(arg0, argv...)
	cmd.Stdout = io.MultiWriter(&stdout, GinkgoWriter)
	cmd.Stderr = GinkgoWriter
	ExpectWithOffset(1, cmd.Run()).To(Succeed())
	return stdout.String()
}

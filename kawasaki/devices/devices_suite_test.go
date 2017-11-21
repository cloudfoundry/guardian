package devices_test

import (
	"os/exec"
	"os/user"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestDevices(t *testing.T) {
	BeforeEach(func() {
		if u, err := user.Current(); err == nil && u.Uid != "0" {
			Skip("Devices suite requires root to run")
		}
	})

	RegisterFailHandler(Fail)
	RunSpecs(t, "Devices Suite")
}

func startCommand(arg0 string, argv ...string) *exec.Cmd {
	cmd := exec.Command(arg0, argv...)
	cmd.Stdout = GinkgoWriter
	cmd.Stderr = GinkgoWriter
	Expect(cmd.Start()).To(Succeed())
	return cmd
}

func runCommand(arg0 string, argv ...string) {
	Expect(startCommand(arg0, argv...).Wait()).To(Succeed())
}

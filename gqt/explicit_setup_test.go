package gqt_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("gdn setup", func() {
	var cgroupsMountpoint string

	BeforeEach(func() {
		cgroupsMountpoint = filepath.Join(os.TempDir(), fmt.Sprintf("cgroups-%d", GinkgoParallelNode()))

		args := []string{"setup", "--tag", fmt.Sprintf("%d", GinkgoParallelNode())}
		setupProcess, err := gexec.Start(exec.Command(gardenBin, args...), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(setupProcess).Should(gexec.Exit(0))
	})

	AfterEach(func() {
		umountCmd := exec.Command("sh", "-c", fmt.Sprintf("umount %s/*", cgroupsMountpoint))
		Expect(umountCmd.Run()).To(Succeed(), "unmount %s", cgroupsMountpoint)
		umountCmd = exec.Command("sh", "-c", fmt.Sprintf("umount %s", cgroupsMountpoint))
		Expect(umountCmd.Run()).To(Succeed(), "unmount %s", cgroupsMountpoint)
	})

	It("sets up cgroups", func() {
		mountpointCmd := exec.Command("mountpoint", "-q", cgroupsMountpoint+"/")
		Expect(mountpointCmd.Run()).To(Succeed())
	})

	It("sets up iptables", func() {})
})

var _ = Describe("running gdn setup before starting server", func() {
	var client *runner.RunningGarden

	BeforeEach(func() {
		setupProcess, err := gexec.Start(exec.Command(gardenBin, "setup"), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(setupProcess).Should(gexec.Exit(0))
		client = startGarden("server")
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	It("creates a container", func() {
		_, err := client.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())
	})
})

package gqt_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"code.cloudfoundry.org/guardian/gqt/runner"
	"code.cloudfoundry.org/idmapper"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("rootless containers", func() {
	var (
		client            *runner.RunningGarden
		maxUID            uint32
		cgroupsMountpoint string
		iptablesPrefix    string
	)

	BeforeEach(func() {
		tag := nodeToString(GinkgoParallelNode())
		cgroupsMountpoint = filepath.Join(os.TempDir(), fmt.Sprintf("cgroups-%s", tag))
		iptablesPrefix = fmt.Sprintf("w-%s", tag)

		setupArgs := []string{"setup", "--tag", fmt.Sprintf("%s", tag)}
		setupProcess, err := gexec.Start(exec.Command(gardenBin, setupArgs...), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(setupProcess).Should(gexec.Exit(0))

		maxUID = uint32(idmapper.Min(idmapper.MustGetMaxValidUID(), idmapper.MustGetMaxValidGID()))
		maxUIDUser := &syscall.Credential{Uid: maxUID, Gid: maxUID}
		client = startGardenAsUser(maxUIDUser, "--skip-setup", "--image-plugin", testImagePluginBin, "--tag", tag)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
		Expect(cleanupSystemResources(cgroupsMountpoint, iptablesPrefix)).To(Succeed())
	})

	Describe("the server process", func() {
		It("can run consistently as a non-root user", func() {
			out, err := exec.Command("ps", "-U", fmt.Sprintf("%d", maxUID)).CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "No process of user maximus was found")
			Expect(out).To(ContainSubstring(fmt.Sprintf("%d", client.Pid)))

			Consistently(func() error {
				return exec.Command("ps", "-p", strconv.Itoa(client.Pid)).Run()
			}).Should(Succeed())
		})
	})

})

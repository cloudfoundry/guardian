package gqt_test

import (
	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("rootless containers", func() {
	var (
		client            *runner.RunningGarden
		cgroupsMountpoint string
		iptablesPrefix    string
	)

	BeforeEach(func() {
		rootlessRuncPath := os.Getenv("ROOTLESS_RUNC_PATH")
		if rootlessRuncPath == "" {
			Fail("ROOTLESS_RUNC_PATH env var is not set")
		}

		tag := nodeToString(GinkgoParallelNode())
		cgroupsMountpoint = filepath.Join(os.TempDir(), fmt.Sprintf("cgroups-%s", tag))
		iptablesPrefix = fmt.Sprintf("w-%s", tag)

		setupArgs := []string{"setup", "--tag", fmt.Sprintf("%s", tag)}
		setupProcess, err := gexec.Start(exec.Command(gardenBin, setupArgs...), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(setupProcess).Should(gexec.Exit(0))

		unprivilegedUser := &syscall.Credential{Uid: unprivilegedUID, Gid: unprivilegedUID}
		unprivilegedUidGid := fmt.Sprintf("%d:%d", unprivilegedUID, unprivilegedUID)

		imagePath, err := ioutil.TempDir("", "rootlessImagePath")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(filepath.Join(imagePath, "image.json"), []byte("{}"), 0777)).To(Succeed())

		// so much easier to just shell out to the OS here ...
		Expect(exec.Command("cp", "-r", os.Getenv("GARDEN_TEST_ROOTFS"), imagePath).Run()).To(Succeed())
		Expect(exec.Command("chown", "-R", unprivilegedUidGid, imagePath).Run()).To(Succeed())

		client = startGardenAsUser(
			unprivilegedUser,
			"--skip-setup",
			"--runc-bin", rootlessRuncPath,
			"--image-plugin", testImagePluginBin,
			"--image-plugin-extra-arg", "\"--image-path\"",
			"--image-plugin-extra-arg", imagePath,
			"--network-plugin", "/bin/true",
			"--tag", tag,
		)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
		Expect(cleanupSystemResources(cgroupsMountpoint, iptablesPrefix)).To(Succeed())
	})

	Describe("the server process", func() {
		It("can run consistently as a non-root user", func() {
			out, err := exec.Command("ps", "-U", fmt.Sprintf("%d", unprivilegedUID)).CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "No process of unprivileged user was found")
			Expect(out).To(ContainSubstring(fmt.Sprintf("%d", client.Pid)))

			Consistently(func() error {
				return exec.Command("ps", "-p", strconv.Itoa(client.Pid)).Run()
			}).Should(Succeed())
		})
	})

	Describe("creating a container", func() {
		It("succeeds", func() {
			_, err := client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

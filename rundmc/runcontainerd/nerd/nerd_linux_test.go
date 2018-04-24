package nerd_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/nerd"
)

var _ = Describe("Nerd", func() {
	var cnerd *nerd.Nerd

	BeforeEach(func() {
		cnerd = nerd.New(containerdClient, containerdContext)
	})

	Describe("Create", func() {
		AfterEach(func() {
			cnerd.Delete(nil, "test-id")
		})

		It("creates the containerd container by id", func() {
			spec := generateSpec(containerdContext, containerdClient, "test-id")
			Expect(cnerd.Create(nil, "test-id", spec)).To(Succeed())

			containers := listContainers(testConfig.CtrBin, testConfig.Socket)
			Expect(containers).To(ContainSubstring("test-id"))
		})

		It("starts an init process in the container", func() {
			spec := generateSpec(containerdContext, containerdClient, "test-id")
			Expect(cnerd.Create(nil, "test-id", spec)).To(Succeed())

			containers := listProcesses(testConfig.CtrBin, testConfig.Socket, "test-id")
			Expect(containers).To(ContainSubstring("test-id"))
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			spec := generateSpec(containerdContext, containerdClient, "test-id")
			Expect(cnerd.Create(nil, "test-id", spec)).To(Succeed())
		})

		It("deletes the containerd container by id", func() {
			Expect(cnerd.Delete(nil, "test-id")).To(Succeed())

			containers := listContainers(testConfig.CtrBin, testConfig.Socket)
			Expect(containers).NotTo(ContainSubstring("test-id"))
		})
	})

	Describe("State", func() {
		BeforeEach(func() {
			spec := generateSpec(containerdContext, containerdClient, "test-id")
			Expect(cnerd.Create(nil, "test-id", spec)).To(Succeed())
		})

		It("gets the pid and status of a running task", func() {
			pid, status, err := cnerd.State(nil, "test-id")

			Expect(err).NotTo(HaveOccurred())
			Expect(pid).NotTo(BeZero())
			Expect(status).To(Equal(containerd.Running))
		})
	})
})

func createRootfs(modifyRootfs func(string), perm os.FileMode) string {
	var err error
	tmpDir, err := ioutil.TempDir("", "test-rootfs")
	Expect(err).NotTo(HaveOccurred())
	unpackedRootfs := filepath.Join(tmpDir, "unpacked")
	Expect(os.Mkdir(unpackedRootfs, perm)).To(Succeed())
	runCommand(exec.Command("tar", "xf", os.Getenv("GARDEN_TEST_ROOTFS"), "-C", unpackedRootfs))
	Expect(os.Chmod(tmpDir, perm)).To(Succeed())
	modifyRootfs(unpackedRootfs)
	return unpackedRootfs
}

func generateSpec(context context.Context, client *containerd.Client, containerID string) *specs.Spec {
	spec, err := oci.GenerateSpec(context, client, &containers.Container{ID: containerID})
	Expect(err).NotTo(HaveOccurred())
	spec.Process.Args = []string{"sleep", "60"}
	spec.Root = &specs.Root{
		Path: createRootfs(func(_ string) {}, 0755),
	}

	return spec
}

func listContainers(ctr, socket string) string {
	return runCtr(ctr, socket, []string{"containers", "list"})
}

func listProcesses(ctr, socket, containerID string) string {
	return runCtr(ctr, socket, []string{"tasks", "ps", containerID})
}

func runCtr(ctr, socket string, args []string) string {
	defaultArgs := []string{"--address", socket, "--namespace", fmt.Sprintf("nerdspace%d", GinkgoParallelNode())}
	cmd := exec.Command(ctr, append(defaultArgs, args...)...)

	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(session).Should(gexec.Exit(0))

	return string(session.Out.Contents())
}

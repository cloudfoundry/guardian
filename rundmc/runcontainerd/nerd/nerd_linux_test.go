package nerd_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
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

	Describe("Delete", func() {
		It("deletes the containerd container by id", func() {
			spec := generateSpec(containerdContext, containerdClient, "test-id")
			container := createContainer(containerdContext, containerdClient, spec, "test-id")
			startInitProcess(containerdContext, container)

			Expect(cnerd.Delete(nil, "test-id")).To(Succeed())
			containers := listContainers(testConfig.CtrBin, testConfig.Socket)
			Expect(containers).NotTo(ContainSubstring("test-id"))
		})
	})

	Describe("State", func() {
		It("gets the pid and status of a running task", func() {
			spec := generateSpec(containerdContext, containerdClient, "test-id")
			container := createContainer(containerdContext, containerdClient, spec, "test-id")
			startInitProcess(containerdContext, container)

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

func createContainer(context context.Context, client *containerd.Client, spec *specs.Spec, containerID string) containerd.Container {
	container, err := client.NewContainer(context, containerID, containerd.WithSpec(spec))
	Expect(err).NotTo(HaveOccurred())
	return container
}

func startInitProcess(context context.Context, container containerd.Container) {
	task, err := container.NewTask(context, cio.NullIO)
	Expect(err).NotTo(HaveOccurred())
	Expect(task.Start(context)).To(Succeed())
}

func listContainers(ctr, socket string) string {
	cmd := exec.Command(ctr, "--address", socket, "--namespace", fmt.Sprintf("nerdspace%d", GinkgoParallelNode()), "containers", "list")

	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(session).Should(gexec.Exit(0))

	return string(session.Out.Contents())
}

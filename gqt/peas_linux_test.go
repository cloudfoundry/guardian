package gqt_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Partially shared containers (peas)", func() {
	var (
		gdn                   *runner.RunningGarden
		almostEmptyRootfsPath string
	)

	BeforeEach(func() {
		gdn = runner.Start(config)
		var err error
		almostEmptyRootfsPath, err = ioutil.TempDir("", "peas-gqts")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Chmod(almostEmptyRootfsPath, 0777)).To(Succeed())

		Expect(copyFile(
			filepath.Join(defaultTestRootFS, "bin", "cat"),
			filepath.Join(almostEmptyRootfsPath, "cat"),
		)).To(Succeed())
		Expect(ioutil.WriteFile(filepath.Join(almostEmptyRootfsPath, "afile"), []byte("hello"), 0666)).To(Succeed())
	})

	AfterEach(func() {
		Expect(gdn.DestroyAndStop()).To(Succeed())
		Expect(os.RemoveAll(almostEmptyRootfsPath)).To(Succeed())
	})

	It("runs a process in its own mount namespace, sharing all other namespaces", func() {
		ctr, err := gdn.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())
		proc, err := ctr.Run(garden.ProcessSpec{
			Path:  "sleep",
			Args:  []string{"60"},
			Image: garden.ImageRef{URI: "raw://" + defaultTestRootFS},
		}, garden.ProcessIO{
			Stdout: GinkgoWriter,
			Stderr: GinkgoWriter,
		})
		Expect(err).NotTo(HaveOccurred())

		ctrInitPid := readFile(filepath.Join(gdn.DepotDir, ctr.Handle(), "pidfile"))
		sleepProcessPidfilePath := filepath.Join(gdn.DepotDir, ctr.Handle(), "processes", proc.ID(), "pidfile")
		Eventually(func() error {
			_, err := os.Stat(sleepProcessPidfilePath)
			return err
		}).Should(Succeed())
		sleepProcessPid := readFile(sleepProcessPidfilePath)

		Expect(getNS(sleepProcessPid, "mnt")).NotTo(Equal(getNS(ctrInitPid, "mnt")))
		for _, ns := range []string{"net", "ipc", "pid", "user", "uts"} {
			Expect(getNS(sleepProcessPid, ns)).To(Equal(getNS(ctrInitPid, ns)))
		}
	})

	It("runs a process with its own rootfs", func() {
		ctr, err := gdn.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())

		stdout := gbytes.NewBuffer()
		_, err = ctr.Run(garden.ProcessSpec{
			Path:  "/cat",
			Args:  []string{"/afile"},
			Image: garden.ImageRef{URI: "raw://" + almostEmptyRootfsPath},
		}, garden.ProcessIO{
			Stdout: io.MultiWriter(stdout, GinkgoWriter),
			Stderr: GinkgoWriter,
		})
		Expect(err).NotTo(HaveOccurred())

		Eventually(stdout).Should(gbytes.Say("hello"))
	})

	It("Process.Wait() returns the process exit code", func() {
		ctr, err := gdn.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())
		proc, err := ctr.Run(garden.ProcessSpec{
			Path:  "sh",
			Args:  []string{"-c", "exit 123"},
			Image: garden.ImageRef{URI: "raw://" + defaultTestRootFS},
		}, garden.ProcessIO{
			Stdin:  bytes.NewBuffer(nil),
			Stdout: GinkgoWriter,
			Stderr: GinkgoWriter,
		})
		Expect(err).NotTo(HaveOccurred())

		procExitCode, err := proc.Wait()
		Expect(err).NotTo(HaveOccurred())
		Expect(procExitCode).To(Equal(123))
	})

	It("client receives stdout and stderr of pea process", func() {
		ctr, err := gdn.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())

		var stdout, stderr bytes.Buffer
		proc, err := ctr.Run(garden.ProcessSpec{
			Path:  "sh",
			Args:  []string{"-c", "echo stdout && echo stderr >&2"},
			Image: garden.ImageRef{URI: "raw://" + defaultTestRootFS},
		}, garden.ProcessIO{
			Stdin:  bytes.NewBuffer(nil),
			Stdout: io.MultiWriter(&stdout, GinkgoWriter),
			Stderr: io.MultiWriter(&stderr, GinkgoWriter),
		})
		Expect(err).NotTo(HaveOccurred())

		procExitCode, err := proc.Wait()
		Expect(err).NotTo(HaveOccurred())
		Expect(procExitCode).To(Equal(0))

		Expect(stdout.String()).To(Equal("stdout\n"))
		Expect(stderr.String()).To(Equal("stderr\n"))
	})
})

func getNS(pid string, ns string) string {
	ns, err := os.Readlink(fmt.Sprintf("/proc/%s/ns/%s", string(pid), ns))
	Expect(err).NotTo(HaveOccurred())
	return ns
}

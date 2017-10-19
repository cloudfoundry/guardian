package gqt_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Partially shared containers (peas)", func() {
	var (
		gdn       *runner.RunningGarden
		tmpDir    string
		peaRootfs string
		ctr       garden.Container
	)

	BeforeEach(func() {
		gdn = runner.Start(config)
		var err error
		tmpDir, err = ioutil.TempDir("", "peas-gqts")
		Expect(err).NotTo(HaveOccurred())

		Expect(exec.Command("cp", "-a", defaultTestRootFS, tmpDir).Run()).To(Succeed())
		Expect(os.Chmod(tmpDir, 0777)).To(Succeed())
		peaRootfs = filepath.Join(tmpDir, "rootfs")
		Expect(ioutil.WriteFile(filepath.Join(peaRootfs, "ima-pea"), []byte("pea!"), 0644)).To(Succeed())

		ctr, err = gdn.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(gdn.DestroyAndStop()).To(Succeed())
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	It("runs a process in its own mount namespace, sharing all other namespaces", func() {
		proc, err := ctr.Run(garden.ProcessSpec{
			Path:  "sleep",
			Args:  []string{"60"},
			Image: garden.ImageRef{URI: "raw://" + peaRootfs},
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
		Expect(readFileInContainer(ctr, "/ima-pea", "raw://"+peaRootfs)).To(Equal("pea!"))
	})

	It("bind mounts the same /etc/hosts file as the container", func() {
		originalContentsInPea := readFileInContainer(ctr, "/etc/hosts", "raw://"+peaRootfs)
		appendFileInContainer(ctr, "/etc/hosts", "foobar", "raw://"+peaRootfs)
		contentsInPea := readFileInContainer(ctr, "/etc/hosts", "raw://"+peaRootfs)
		Expect(originalContentsInPea).NotTo(Equal(contentsInPea))
		contentsInContainer := readFileInContainer(ctr, "/etc/hosts", "")
		Expect(contentsInPea).To(Equal(contentsInContainer))
	})

	It("bind mounts the same /etc/resolv.conf file as the container", func() {
		originalContentsInPea := readFileInContainer(ctr, "/etc/resolv.conf", "raw://"+peaRootfs)
		appendFileInContainer(ctr, "/etc/resolv.conf", "foobar", "raw://"+peaRootfs)
		contentsInPea := readFileInContainer(ctr, "/etc/resolv.conf", "raw://"+peaRootfs)
		Expect(originalContentsInPea).NotTo(Equal(contentsInPea))
		contentsInContainer := readFileInContainer(ctr, "/etc/resolv.conf", "")
		Expect(contentsInPea).To(Equal(contentsInContainer))
	})

	It("runs the process as the specified uid + gid", func() {
		var stdout bytes.Buffer
		proc, err := ctr.Run(garden.ProcessSpec{
			Path:  "sh",
			Args:  []string{"-c", "echo $(id -u):$(id -g)"},
			Image: garden.ImageRef{URI: "raw://" + peaRootfs},
			User:  "1001:1002",
		}, garden.ProcessIO{
			Stdin:  bytes.NewBuffer(nil),
			Stdout: io.MultiWriter(&stdout, GinkgoWriter),
			Stderr: GinkgoWriter,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(proc.Wait()).To(Equal(0))

		Expect(stdout.String()).To(Equal("1001:1002\n"))
	})

	It("runs the process as the uid 0 and gid 0 unless specified", func() {
		var stdout bytes.Buffer
		proc, err := ctr.Run(garden.ProcessSpec{
			Path:  "sh",
			Args:  []string{"-c", "echo $(id -u):$(id -g)"},
			Image: garden.ImageRef{URI: "raw://" + peaRootfs},
		}, garden.ProcessIO{
			Stdin:  bytes.NewBuffer(nil),
			Stdout: io.MultiWriter(&stdout, GinkgoWriter),
			Stderr: GinkgoWriter,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(proc.Wait()).To(Equal(0))

		Expect(stdout.String()).To(Equal("0:0\n"))
	})

	It("cannot run peas with a username", func() {
		_, err := ctr.Run(garden.ProcessSpec{
			User:  "root",
			Path:  "pwd",
			Image: garden.ImageRef{URI: "raw://" + peaRootfs},
		}, garden.ProcessIO{
			Stdin:  bytes.NewBuffer(nil),
			Stdout: GinkgoWriter,
			Stderr: GinkgoWriter,
		})
		Expect(err).To(HaveOccurred())
	})

	It("Process.Wait() returns the process exit code", func() {
		proc, err := ctr.Run(garden.ProcessSpec{
			Path:  "sh",
			Args:  []string{"-c", "exit 123"},
			Image: garden.ImageRef{URI: "raw://" + peaRootfs},
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
		var stdout, stderr bytes.Buffer
		proc, err := ctr.Run(garden.ProcessSpec{
			Path:  "sh",
			Args:  []string{"-c", "echo stdout && echo stderr >&2"},
			Image: garden.ImageRef{URI: "raw://" + peaRootfs},
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

func readFileInContainer(ctr garden.Container, pathname string, image string) string {
	stdout := bytes.Buffer{}
	proc, err := ctr.Run(garden.ProcessSpec{
		Path:  "cat",
		Args:  []string{pathname},
		Image: garden.ImageRef{URI: image},
	}, garden.ProcessIO{
		Stdin:  bytes.NewBuffer(nil),
		Stdout: io.MultiWriter(&stdout, GinkgoWriter),
		Stderr: GinkgoWriter,
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(proc.Wait()).To(Equal(0))
	return stdout.String()
}

func appendFileInContainer(ctr garden.Container, pathname, toAppend, image string) {
	proc, err := ctr.Run(garden.ProcessSpec{
		Path:  "sh",
		Args:  []string{"-c", fmt.Sprintf("echo %s >> %s", toAppend, pathname)},
		Image: garden.ImageRef{URI: image},
	}, garden.ProcessIO{
		Stdin:  bytes.NewBuffer(nil),
		Stdout: GinkgoWriter,
		Stderr: GinkgoWriter,
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(proc.Wait()).To(Equal(0))
}

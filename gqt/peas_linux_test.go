package gqt_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Partially shared containers (peas)", func() {
	var (
		gdn           *runner.RunningGarden
		peaRootfs     string
		ctr           garden.Container
		containerSpec garden.ContainerSpec
	)

	BeforeEach(func() {
		peaRootfs = createPeaRootfs()
		containerSpec = garden.ContainerSpec{}
	})

	JustBeforeEach(func() {
		gdn = runner.Start(config)
		var err error
		ctr, err = gdn.Create(containerSpec)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(gdn.DestroyAndStop()).To(Succeed())
	})

	It("should not leak pipes", func() {
		initialPipes := numPipes(gdn.Pid)

		process, err := ctr.Run(garden.ProcessSpec{
			Path:  "echo",
			Args:  []string{"hello"},
			Image: garden.ImageRef{URI: "raw://" + peaRootfs},
		}, garden.ProcessIO{})
		Expect(err).NotTo(HaveOccurred())
		Expect(process.Wait()).To(Equal(0))

		Eventually(func() int { return numPipes(gdn.Pid) }).Should(Equal(initialPipes))
	})

	Describe("process limits", func() {
		It("should not leak cgroups", func() {
			stdout := gbytes.NewBuffer()
			process, err := ctr.Run(garden.ProcessSpec{
				ID:    "pea-process",
				Path:  "cat",
				Args:  []string{"/proc/self/cgroup"},
				Image: garden.ImageRef{URI: "raw://" + peaRootfs},
				OverrideContainerLimits: &garden.ProcessLimits{},
			}, garden.ProcessIO{
				Stdout: stdout,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(process.Wait()).To(Equal(0))

			cgroupsPath := filepath.Join(config.TmpDir, fmt.Sprintf("cgroups-%s", config.Tag))

			peaCgroups := stdout.Contents()
			cgroups := strings.Split(string(peaCgroups), "\n")
			for _, cgroup := range cgroups[:len(cgroups)-1] {
				cgroupData := strings.Split(cgroup, ":")
				Eventually(filepath.Join(cgroupsPath, cgroupData[1], cgroupData[2])).ShouldNot(BeADirectory())
			}
		})

		Context("when a process with cpu limits is created", func() {
			var cgroupPath string

			JustBeforeEach(func() {
				stdout := gbytes.NewBuffer()
				_, err := ctr.Run(garden.ProcessSpec{
					Path:  "sh",
					Args:  []string{"-c", "cat /proc/self/cgroup && echo done && sleep 3600"},
					Image: garden.ImageRef{URI: "raw://" + peaRootfs},
					OverrideContainerLimits: &garden.ProcessLimits{
						CPU: garden.CPULimits{LimitInShares: 128},
					},
				}, garden.ProcessIO{
					Stdout: io.MultiWriter(stdout, GinkgoWriter),
					Stderr: GinkgoWriter,
				})
				Expect(err).NotTo(HaveOccurred())
				Eventually(stdout).Should(gbytes.Say("done"))

				firstCgroupProcLine := strings.Split(string(stdout.Contents()), "\n")[0]
				cgroupRelativePath := strings.Split(firstCgroupProcLine, ":")[2]
				cgroupPath = filepath.Join(config.TmpDir, fmt.Sprintf("cgroups-%s", config.Tag),
					"cpu", cgroupRelativePath)
			})

			Context("when started with low cpu limit turned on", func() {
				BeforeEach(func() {
					config.CPUQuotaPerShare = uint64ptr(10)
				})

				It("throttles process cpu usage", func() {
					periods, throttled, time, err := parseCpuStats(filepath.Join(cgroupPath, "cpu.stat"))
					Expect(err).NotTo(HaveOccurred())
					Expect(periods).To(BeNumerically(">", 0))
					Expect(throttled).To(BeNumerically(">", 0))
					Expect(time).To(BeNumerically(">", 0))
				})

				It("sets cpu.cfs_period_us to 100000 (100ms)", func() {
					period := readFile(filepath.Join(cgroupPath, "cpu.cfs_period_us"))
					Expect(strings.TrimSpace(period)).To(Equal("100000"))
				})

				It("configures cpu.cfs_quota_us as shares * cpu-quota-per-share", func() {
					period := readFile(filepath.Join(cgroupPath, "cpu.cfs_quota_us"))
					Expect(strings.TrimSpace(period)).To(Equal("1280"))
				})
			})

			Context("when started with low cpu limit turned off", func() {
				It("does not throttle process cpu usage", func() {
					periods, throttled, time, err := parseCpuStats(filepath.Join(cgroupPath, "cpu.stat"))
					Expect(err).NotTo(HaveOccurred())
					Expect(periods).To(BeNumerically("==", 0))
					Expect(throttled).To(BeNumerically("==", 0))
					Expect(time).To(BeNumerically("==", 0))
				})

				It("configures cpu.cfs_quota_us as shares * cpu-quota-per-share", func() {
					period := readFile(filepath.Join(cgroupPath, "cpu.cfs_quota_us"))
					Expect(strings.TrimSpace(period)).To(Equal("-1"))
				})
			})
		})
	})

	Describe("Process dir", func() {
		var processPath string

		JustBeforeEach(func() {
			process, err := ctr.Run(garden.ProcessSpec{
				Path:  "echo",
				Args:  []string{"hello"},
				Image: garden.ImageRef{URI: "raw://" + peaRootfs},
			}, garden.ProcessIO{})
			Expect(err).NotTo(HaveOccurred())
			Expect(process.Wait()).To(Equal(0))
			processPath = filepath.Join(gdn.DepotDir, ctr.Handle(), "processes", process.ID())
		})

		Context("when --cleanup-process-dirs-on-wait is set", func() {
			BeforeEach(func() {
				config.CleanupProcessDirsOnWait = boolptr(true)
			})

			It("should delete pea process dir", func() {
				Expect(processPath).NotTo(BeADirectory())
			})
		})

		Context("when --cleanup-process-dirs-on-wait is not set (default)", func() {
			BeforeEach(func() {
				config.CleanupProcessDirsOnWait = boolptr(false)
			})

			It("should not delete pea process dir", func() {
				Expect(processPath).To(BeADirectory())
			})
		})
	})

	Describe("Bind mounts", func() {
		var testSrcFile *os.File
		destinationFile := "/tmp/file"
		output := gbytes.NewBuffer()

		BeforeEach(func() {
			var err error
			testSrcFile, err = ioutil.TempFile("", "host-file")
			Expect(err).NotTo(HaveOccurred())
			_, err = testSrcFile.WriteString("test-mount")
			Expect(err).NotTo(HaveOccurred())
			Expect(exec.Command("chown", "4294967294", testSrcFile.Name()).Run()).To(Succeed())
		})

		Context("when we create a pea with bind mounts", func() {
			It("should have access to the mounts", func() {
				process, err := ctr.Run(garden.ProcessSpec{
					Path:  "cat",
					Args:  []string{destinationFile},
					Image: garden.ImageRef{URI: "raw://" + peaRootfs},
					BindMounts: []garden.BindMount{
						garden.BindMount{
							SrcPath: testSrcFile.Name(),
							DstPath: destinationFile,
						},
					},
				}, garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, output),
					Stderr: GinkgoWriter,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(process.Wait()).To(Equal(0))
				Expect(output).To(gbytes.Say("test-mount"))
			})
		})

		Context("when there are already bind mounts in the container", func() {
			BeforeEach(func() {
				containerSpec = garden.ContainerSpec{
					BindMounts: []garden.BindMount{
						garden.BindMount{
							SrcPath: testSrcFile.Name(),
							DstPath: destinationFile,
						},
					},
				}
			})

			It("the pea should not have access to the mounts", func() {
				process, err := ctr.Run(garden.ProcessSpec{
					Path:  "cat",
					Args:  []string{destinationFile},
					Image: garden.ImageRef{URI: "raw://" + peaRootfs},
				}, garden.ProcessIO{
					Stdout: GinkgoWriter,
					Stderr: io.MultiWriter(GinkgoWriter, output),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(process.Wait()).To(Equal(1))
				Expect(output).To(gbytes.Say("No such file or directory"))
			})
		})
	})
})

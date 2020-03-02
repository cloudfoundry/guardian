package gqt_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
		Expect(os.RemoveAll(filepath.Dir(peaRootfs))).To(Succeed())
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

	Context("when run with /etc/passwd username", func() {
		BeforeEach(func() {
			config.CleanupProcessDirsOnWait = boolptr(true)
		})

		It("should not leak username resolving peas", func() {
			process, err := ctr.Run(garden.ProcessSpec{
				User:  "alice",
				Path:  "echo",
				Args:  []string{"hello"},
				Image: garden.ImageRef{URI: "raw://" + peaRootfs},
			}, garden.ProcessIO{})
			Expect(err).NotTo(HaveOccurred())

			exitCode, err := process.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(0))
			Expect(collectPeaPids(ctr.Handle())).To(BeEmpty())
		})

		Context("when using runc for peas", func() {
			BeforeEach(func() {
				skipIfContainerdForProcesses("processes directory in the depot is not created for peas run via containerd")
			})

			It("should not leak process dirs", func() {
				numProcessDirsBefore := numProcessDirs(ctr.Handle())

				process, err := ctr.Run(garden.ProcessSpec{
					User:  "alice",
					Path:  "echo",
					Args:  []string{"hello"},
					Image: garden.ImageRef{URI: "raw://" + peaRootfs},
				}, garden.ProcessIO{})
				Expect(err).NotTo(HaveOccurred())
				Expect(process.Wait()).To(Equal(0))

				Expect(numProcessDirs(ctr.Handle())).To(Equal(numProcessDirsBefore))
			})
		})

		Context("when using containerd for peas", func() {
			BeforeEach(func() {
				skipIfRunDmcForProcesses("no containerd in the runc for processes world")
			})

			It("should delete the username resolving pea container", func() {
				containersLsBefore := listContainers("ctr", config.ContainerdSocket)
				process, err := ctr.Run(garden.ProcessSpec{
					User:  "alice",
					Path:  "echo",
					Args:  []string{"hello"},
					Image: garden.ImageRef{URI: "raw://" + peaRootfs},
				}, garden.ProcessIO{})
				Expect(err).NotTo(HaveOccurred())
				Expect(process.Wait()).To(Equal(0))
				Expect(listContainers("ctr", config.ContainerdSocket)).To(Equal(containersLsBefore))
			})
		})

		When("providing a rootfs tar", func() {
			var rootfs string

			BeforeEach(func() {
				rootfs = createPeaRootfsTar()
			})

			AfterEach(func() {
				Expect(os.RemoveAll(filepath.Dir(rootfs))).To(Succeed())
			})

			It("should not leak image dirs", func() {
				numImageDirsBefore := numImageDirs()

				process, err := ctr.Run(garden.ProcessSpec{
					User:  "alice",
					Path:  "echo",
					Args:  []string{"hello"},
					Image: garden.ImageRef{URI: rootfs},
				}, garden.ProcessIO{})
				Expect(err).NotTo(HaveOccurred())
				Expect(process.Wait()).To(Equal(0))

				Expect(numImageDirs()).To(Equal(numImageDirsBefore))
			})
		})
	})

	Describe("process limits", func() {
		FIt("should not leak cgroups", func() {
			stdout := gbytes.NewBuffer()
			process, err := ctr.Run(garden.ProcessSpec{
				ID:                      "pea-process",
				Path:                    "cat",
				Args:                    []string{"/proc/self/cgroup"},
				Image:                   garden.ImageRef{URI: "raw://" + peaRootfs},
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

				cgroupProcLines := strings.Split(string(stdout.Contents()), "\n")
				var cgroupRelativePath string
				for _, procLine := range cgroupProcLines {
					procLineSections := strings.Split(procLine, ":")
					if procLineSections[1] == "memory" {
						cgroupRelativePath = procLineSections[2]
						break
					}
				}
				cgroupPath = filepath.Join(gdn.CgroupsRootPath(),
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
					period := readFileString(filepath.Join(cgroupPath, "cpu.cfs_period_us"))
					Expect(strings.TrimSpace(period)).To(Equal("100000"))
				})

				It("configures cpu.cfs_quota_us as shares * cpu-quota-per-share", func() {
					period := readFileString(filepath.Join(cgroupPath, "cpu.cfs_quota_us"))
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
					period := readFileString(filepath.Join(cgroupPath, "cpu.cfs_quota_us"))
					Expect(strings.TrimSpace(period)).To(Equal("-1"))
				})
			})
		})
	})

	Describe("Pea process cleanup", func() {
		var (
			process     garden.Process
			processPath string
		)

		JustBeforeEach(func() {
			var err error
			process, err = ctr.Run(garden.ProcessSpec{
				Path:  "echo",
				Args:  []string{"hello"},
				Image: garden.ImageRef{URI: "raw://" + peaRootfs},
			}, garden.ProcessIO{})
			Expect(err).NotTo(HaveOccurred())
			Expect(process.Wait()).To(Equal(0))
			processPath = filepath.Join(gdn.DepotDir, ctr.Handle(), "processes", process.ID())
		})

		Context("when using containerd for processes", func() {
			BeforeEach(func() {
				skipIfRunDmcForProcesses("There is no containers in containerd when processes are run via rundmc")
			})

			It("should delete the backing containerd container", func() {
				Expect(runCtr("ctr", config.ContainerdSocket, []string{"containers", "ls"})).NotTo(ContainSubstring(process.ID()))
			})
		})

		Context("when using runc for processes", func() {
			BeforeEach(func() {
				skipIfContainerdForProcesses("There is no process directory in the depot when processes are run via containerd")
			})

			// It("should not delete pea process dir", func() {
			// 	Expect(processPath).To(BeADirectory())
			// })

			Context("when --cleanup-process-dirs-on-wait is set", func() {
				BeforeEach(func() {
					config.CleanupProcessDirsOnWait = boolptr(true)
				})

				It("should delete pea process dir", func() {
					Expect(processPath).NotTo(BeADirectory())
				})
			})
		})
	})

	Describe("Bind mounts", func() {
		var testSrcFile *os.File
		destinationFile := "/tmp/file"
		output := gbytes.NewBuffer()

		BeforeEach(func() {
			testSrcFile = tempFile("", "host-file")
			_, err := testSrcFile.WriteString("test-mount")
			Expect(err).NotTo(HaveOccurred())
			Expect(exec.Command("chown", "4294967294", testSrcFile.Name()).Run()).To(Succeed())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(testSrcFile.Name())).To(Succeed())
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

	Context("when creating a pea fails", func() {
		It("should cleanup the process dir", func() {
			_, err := ctr.Run(garden.ProcessSpec{
				ID:    "failingPea",
				Path:  "nonexistent",
				Image: garden.ImageRef{URI: "raw://" + peaRootfs},
			}, garden.ProcessIO{})
			Expect(err).To(MatchError(ContainSubstring("executable file not found")))

			Expect(filepath.Join(config.DepotDir, ctr.Handle(), "processes", "failingPea")).NotTo(BeADirectory())
		})
	})

	Context("Metrics", func() {
		When("the runtime plugin does not return created time", func() {
			BeforeEach(func() {
				// Containerd does not work well with the fake runtime plugin
				skipIfContainerd()

				config.RuntimePluginBin = binaries.RuntimePlugin
				config.NetworkPluginBin = binaries.NetworkPlugin
				config.ImagePluginBin = binaries.NoopPlugin
			})

			It("returns the age of the pea", func() {
				process, err := ctr.Run(garden.ProcessSpec{
					Path:  "echo",
					Image: garden.ImageRef{URI: "fake-rootfs"},
				}, garden.ProcessIO{})
				Expect(err).NotTo(HaveOccurred())

				metrics, err := gdn.BulkMetrics([]string{process.ID()})
				Expect(err).NotTo(HaveOccurred())
				Expect(metrics).To(HaveKey(process.ID()))
				Expect(metrics[process.ID()].Err).To(BeNil())
				Expect(metrics[process.ID()].Metrics.Age).NotTo(BeZero())
			})
		})
	})
})

func collectPeaPids(handle string) []string {
	if isContainerdForProcesses() {
		return collectContainerdPeaPids(handle)
	}

	return collectRuncPeaPids(handle)
}

func collectContainerdPeaPids(handle string) []string {
	allPids := getContainerPids("ctr", config.ContainerdSocket, handle)
	if len(allPids) == 0 {
		return []string{}
	}
	sort.Strings(allPids)
	// Filter out the init process PID (the init process pid is the lowest among all container pids)
	return allPids[1:]
}

func collectRuncPeaPids(handle string) []string {
	peaPids := []string{}
	processesDir := filepath.Join(config.DepotDir, handle, "processes")

	err := filepath.Walk(processesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}
		if info.Name() == "pidfile" {
			peaPids = append(peaPids, string(readFile(path)))
		}
		return nil
	})
	Expect(err).NotTo(HaveOccurred())
	return peaPids
}

func numProcessDirs(handle string) int {
	processDirs, err := ioutil.ReadDir(filepath.Join(config.DepotDir, handle, "processes"))
	if os.IsNotExist(err) {
		return 0
	}
	Expect(err).ToNot(HaveOccurred())
	return len(processDirs)
}

func numImageDirs() int {
	imagesDirs, err := ioutil.ReadDir(filepath.Join(config.StorePath, "images"))
	Expect(err).ToNot(HaveOccurred())
	return len(imagesDirs)
}

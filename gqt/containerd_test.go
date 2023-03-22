package gqt_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	uuid "github.com/nu7hatch/gouuid"
)

var _ = Describe("Containerd", func() {
	var client *runner.RunningGarden

	BeforeEach(func() {
		skipIfNotContainerd()
	})

	JustBeforeEach(func() {
		client = runner.Start(config)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	Describe("creating containers", func() {
		It("creates a containerd container with running init task", func() {
			container, err := client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			containers := listContainers("ctr", config.ContainerdSocket)
			Expect(containers).To(ContainSubstring(container.Handle()))

			tasks := listTasks("ctr", config.ContainerdSocket)
			Expect(tasks).To(ContainSubstring(container.Handle()))
			Expect(tasks).To(MatchRegexp(container.Handle() + `\s+\d+\s+RUNNING`))
		})

		When("containerd does not manage to get the task state", func() {
			var freezerCgroupPath string

			JustBeforeEach(func() {
				var err error
				cgroupsPath := filepath.Join("/tmp", fmt.Sprintf("cgroups-%s", config.Tag), "freezer")
				freezerCgroupPath, err = ioutil.TempDir(cgroupsPath, "shim")
				Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				Expect(client.Destroy("BOO")).To(Succeed())
				Expect(syscall.Rmdir(freezerCgroupPath)).To(Succeed())
			})

			It("doesn't return a misleading error", func() {
				container, err := client.Create(garden.ContainerSpec{Handle: "BOO"})
				Expect(err).NotTo(HaveOccurred())

				allPids := getContainerPids("ctr", config.ContainerdSocket, "BOO")
				Expect(allPids).To(HaveLen(1))

				parentPid := getParentPid(allPids[0])

				freezerProcs := filepath.Join(freezerCgroupPath, "cgroup.procs")
				Expect(ioutil.WriteFile(freezerProcs, []byte(parentPid), 0755)).To(Succeed())
				freezerState := filepath.Join(freezerCgroupPath, "freezer.state")
				Expect(ioutil.WriteFile(freezerState, []byte("FROZEN"), 0755)).To(Succeed())

				defer func() {
					Expect(ioutil.WriteFile(freezerState, []byte("THAWED"), 0755)).To(Succeed())
				}()

				Eventually(func() string {
					state, err := ioutil.ReadFile(freezerState)
					Expect(err).NotTo(HaveOccurred())
					return string(state)
				}).Should(ContainSubstring("FROZEN"))

				_, infoErr := container.Info()
				Expect(infoErr).To(MatchError("failed getting task"))

				By("retrying a reasonable number of times", func() {
					for i := 1; i <= 5; i++ {
						Expect(client).To(gbytes.Say("retrying"), fmt.Sprintf("Failed waiting for #%d retry", i))
					}
				})
			})
		})
	})

	Describe("destroying a container", func() {
		var container garden.Container

		JustBeforeEach(func() {
			var err error
			container, err = client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("removes the container from ctr lookup", func() {
			err := client.Destroy(container.Handle())
			Expect(err).NotTo(HaveOccurred())

			containers := listContainers("ctr", config.ContainerdSocket)
			Expect(containers).NotTo(ContainSubstring(container.Handle()))
		})
	})

	Describe("running a process in a container", func() {
		var (
			processID string
			container garden.Container
		)

		JustBeforeEach(func() {
			var err error
			container, err = client.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(client.Destroy(container.Handle())).To(Succeed())
		})

		It("succeeds", func() {
			process, err := container.Run(garden.ProcessSpec{
				Path: "/bin/sh",
				Args: []string{"-c", "exit 17"},
			}, garden.ProcessIO{})
			Expect(err).NotTo(HaveOccurred())
			statusCode, err := process.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(statusCode).To(Equal(17))
		})

		It("can attach to a process", func() {
			process, err := container.Run(garden.ProcessSpec{
				Path: "/bin/sh",
				Args: []string{"-c", "exit 13"},
			}, garden.ProcessIO{})
			Expect(err).NotTo(HaveOccurred())
			processID = process.ID()

			attachedProcess, err := container.Attach(processID, garden.ProcessIO{})
			Expect(err).NotTo(HaveOccurred())

			exitCode, err := attachedProcess.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(13))
		})

		When("use_containerd_for_processes is enabled", func() {
			BeforeEach(func() {
				config.UseContainerdForProcesses = boolptr(true)
			})

			It("is known about by containerd", func() {
				_, err := container.Run(garden.ProcessSpec{
					ID:   "ctrd-process-id",
					Path: "/bin/sleep",
					Args: []string{"10"},
					Dir:  "/",
				}, garden.ProcessIO{})
				Expect(err).NotTo(HaveOccurred())

				processes := listProcesses("ctr", config.ContainerdSocket, container.Handle())
				Expect(processes).To(ContainSubstring("ctrd-process-id"))
			})

			It("can resolve the user of the process", func() {
				stdout := gbytes.NewBuffer()
				_, err := container.Run(garden.ProcessSpec{
					ID:   "ctrd-process-id",
					Path: "/bin/ps",
					User: "1000",
					Dir:  "/",
				}, garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, stdout),
				})
				Expect(err).NotTo(HaveOccurred())
				Eventually(stdout).Should(gbytes.Say("alice"))
			})

			It("can resolve the home directory of the user if none was specified", func() {
				stdout := gbytes.NewBuffer()
				_, err := container.Run(garden.ProcessSpec{
					ID:   "ctrd-process-pwd",
					Path: "/bin/pwd",
					User: "alice",
				}, garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, stdout),
				})
				Expect(err).NotTo(HaveOccurred())

				Eventually(stdout).Should(gbytes.Say("/home/alice"))
			})

			It("can run a process without providing an ID", func() {
				stdout := gbytes.NewBuffer()
				_, err := container.Run(garden.ProcessSpec{
					Path: "/bin/echo",
					Args: []string{"hello alice"},
				}, garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, stdout),
				})
				Expect(err).NotTo(HaveOccurred())
				Eventually(stdout).Should(gbytes.Say("hello alice"))
			})

			It("returns the correct process ID", func() {
				proc, err := container.Run(garden.ProcessSpec{
					ID:   "some-process-id",
					Path: "/bin/echo",
				}, garden.ProcessIO{})
				Expect(err).NotTo(HaveOccurred())
				Expect(proc.ID()).To(Equal("some-process-id"))
			})

			Describe("Wait", func() {
				var (
					process   garden.Process
					processID string
				)

				JustBeforeEach(func() {
					// We can get rid of specifying the ID when https://www.pivotaltracker.com/story/show/162442843 gets fixed
					randomID, err := uuid.NewV4()
					Expect(err).NotTo(HaveOccurred())
					processID = randomID.String()

					process, err = container.Run(garden.ProcessSpec{
						ID:   processID,
						Path: "/bin/sh",
						Args: []string{"-c", "exit 17"},
					}, garden.ProcessIO{})
					Expect(err).NotTo(HaveOccurred())
				})

				It("can get the exit code of a process", func() {
					Expect(process.Wait()).To(Equal(17))
				})

				It("can call Wait more than once on the same process", func() {
					Expect(process.Wait()).To(Equal(17))
					Expect(process.Wait()).To(Equal(17))
				})

				Context("when cleanup-process-dirs-on-wait is enabled", func() {
					BeforeEach(func() {
						config.CleanupProcessDirsOnWait = boolptr(true)
					})

					It("deletes all state files for that process", func() {
						Expect(process.Wait()).To(Equal(17))

						Expect(findFilesContaining(processID)).To(BeFalse())
					})
				})
			})

			Describe("Stdio", func() {
				It("connects stdin", func() {
					stdout := gbytes.NewBuffer()
					stdin := bytes.NewBufferString("hello from stdin")
					process, err := container.Run(garden.ProcessSpec{
						Path: "cat",
					}, garden.ProcessIO{
						Stdin:  stdin,
						Stdout: io.MultiWriter(GinkgoWriter, stdout),
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(process.Wait()).To(Equal(0))

					Eventually(stdout).Should(gbytes.Say("hello from stdin"))
				})

				It("connects stdout", func() {
					stdout := new(bytes.Buffer)
					process, err := container.Run(garden.ProcessSpec{
						Path: "/bin/echo",
						Args: []string{"-n", "hello world"},
					}, garden.ProcessIO{
						Stdout: io.MultiWriter(GinkgoWriter, stdout),
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(process.Wait()).To(Equal(0))

					Expect(stdout.String()).To(Equal("hello world"))
				})

				It("connects stderr", func() {
					stderr := new(bytes.Buffer)
					process, err := container.Run(garden.ProcessSpec{
						Path: "/bin/sh",
						Args: []string{"-c", "/bin/echo -n hello error 1>&2"},
					}, garden.ProcessIO{
						Stderr: io.MultiWriter(GinkgoWriter, stderr),
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(process.Wait()).To(Equal(0))

					Expect(stderr.String()).To(Equal("hello error"))
				})
			})

			Describe("Signalling", func() {
				It("forwards signals to the process", func() {
					buffer := gbytes.NewBuffer()
					proc, err := container.Run(garden.ProcessSpec{
						Path: "sh",
						Args: []string{"-c", `
					trap 'exit 42' TERM

					while true; do
					  echo 'sleeping'
					  sleep 1
					done
				`},
					}, garden.ProcessIO{
						Stdout: buffer,
					})
					Expect(err).NotTo(HaveOccurred())

					Eventually(buffer).Should(gbytes.Say("sleeping"))

					err = proc.Signal(garden.SignalTerminate)
					Expect(err).NotTo(HaveOccurred())

					status := make(chan int)
					go func() {
						exit, err := proc.Wait()
						Expect(err).NotTo(HaveOccurred())
						status <- exit
					}()

					Eventually(status).Should(Receive(BeEquivalentTo(42)))
				})
			})

			Describe("pea", func() {
				var rootfs string

				BeforeEach(func() {
					rootfs = createPeaRootfsTar()
				})

				AfterEach(func() {
					Expect(os.RemoveAll(filepath.Dir(rootfs))).To(Succeed())
				})

				It("creates a containerd container with a running task", func() {
					process, err := container.Run(garden.ProcessSpec{
						ID:    "ctrd-pea-id",
						Image: garden.ImageRef{URI: rootfs},
						Path:  "/bin/sleep",
						Args:  []string{"10"},
						User:  "alice",
					}, ginkgoIO)
					Expect(err).NotTo(HaveOccurred())

					containers := listContainers("ctr", config.ContainerdSocket)
					Expect(containers).To(ContainSubstring("ctrd-pea-id"))

					pids := getContainerPids("ctr", config.ContainerdSocket, "ctrd-pea-id")
					Expect(pids).To(HaveLen(1))

					cmdline := readFileString(filepath.Join("/", "proc", pids[0], "cmdline"))
					Expect(cmdline).To(ContainSubstring("/bin/sleep"))

					code, err := process.Wait()
					Expect(err).NotTo(HaveOccurred())
					Expect(code).To(Equal(0))
				})

				It("cleans up pea-debris", func() {
					process, err := container.Run(garden.ProcessSpec{
						ID:    "ctrd-pea-id-2",
						Image: garden.ImageRef{URI: rootfs},
						Path:  "/bin/echo",
						Args:  []string{"peeeeee"},
						User:  "alice",
					}, garden.ProcessIO{})
					Expect(err).NotTo(HaveOccurred())

					code, err := process.Wait()
					Expect(err).NotTo(HaveOccurred())
					Expect(code).To(Equal(0))

					tasks := listTasks("ctr", config.ContainerdSocket)
					Expect(tasks).NotTo(ContainSubstring("ctrd-pea-id-2"))
					Expect(tasks).To(MatchRegexp(container.Handle() + `\s+\d+\s+RUNNING`))

					containers := listContainers("ctr", config.ContainerdSocket)
					Expect(containers).NotTo(ContainSubstring("ctrd-pea-id-2"))
				})

				It("returns the process exit code", func() {
					process, err := container.Run(garden.ProcessSpec{
						Image: garden.ImageRef{URI: rootfs},
						Path:  "/bin/sh",
						Args:  []string{"-c", "exit 12"},
					}, garden.ProcessIO{})
					Expect(err).NotTo(HaveOccurred())

					code, err := process.Wait()
					Expect(err).NotTo(HaveOccurred())
					Expect(code).To(Equal(12))
				})

				Describe("Stdio", func() {
					It("connects stdin", func() {
						stdout := gbytes.NewBuffer()
						stdin := bytes.NewBufferString("hello from stdin")
						process, err := container.Run(garden.ProcessSpec{
							Path:  "cat",
							Image: garden.ImageRef{URI: rootfs},
						}, garden.ProcessIO{
							Stdin:  stdin,
							Stdout: io.MultiWriter(GinkgoWriter, stdout),
						})
						Expect(err).NotTo(HaveOccurred())
						Expect(process.Wait()).To(Equal(0))
						Eventually(stdout).Should(gbytes.Say("hello from stdin"))
					})

					It("connects stdout", func() {
						stdout := new(bytes.Buffer)
						process, err := container.Run(garden.ProcessSpec{
							Path:  "/bin/echo",
							Args:  []string{"-n", "hello world"},
							Image: garden.ImageRef{URI: rootfs},
						}, garden.ProcessIO{
							Stdout: io.MultiWriter(GinkgoWriter, stdout),
						})
						Expect(err).NotTo(HaveOccurred())
						Expect(process.Wait()).To(Equal(0))

						Expect(stdout.String()).To(Equal("hello world"))
					})

					It("connects stderr", func() {
						stderr := new(bytes.Buffer)
						process, err := container.Run(garden.ProcessSpec{
							Path:  "/bin/sh",
							Args:  []string{"-c", "/bin/echo -n hello error 1>&2"},
							Image: garden.ImageRef{URI: rootfs},
						}, garden.ProcessIO{
							Stderr: io.MultiWriter(GinkgoWriter, stderr),
						})
						Expect(err).NotTo(HaveOccurred())
						Expect(process.Wait()).To(Equal(0))

						Expect(stderr.String()).To(Equal("hello error"))
					})
				})

				Describe("Signalling", func() {
					It("forwards signals to the pea", func() {
						buffer := gbytes.NewBuffer()
						proc, err := container.Run(garden.ProcessSpec{
							Path:  "sh",
							Image: garden.ImageRef{URI: rootfs},
							Args: []string{"-c", `
					trap 'exit 42' TERM

					while true; do
					  echo 'sleeping'
					  sleep 1
					done
				`},
						}, garden.ProcessIO{
							Stdout: buffer,
						})
						Expect(err).NotTo(HaveOccurred())

						Eventually(buffer).Should(gbytes.Say("sleeping"))

						err = proc.Signal(garden.SignalTerminate)
						Expect(err).NotTo(HaveOccurred())

						status := make(chan int)
						go func() {
							defer GinkgoRecover()

							exit, err := proc.Wait()
							Expect(err).NotTo(HaveOccurred())
							status <- exit
						}()

						timeout := time.NewTimer(5 * time.Second)
						select {
						case s := <-status:
							Expect(s).To(BeEquivalentTo(42))
						case <-timeout.C:
							Fail("No signal received after 5 seconds!\n\nProcesses running:\n" + psFaux())
						}
					})
				})
			})
		})
	})

	Describe("containerd restart", func() {
		var container garden.Container

		JustBeforeEach(func() {
			restartContainerd(client)
			var err error
			container, err = client.Create(garden.ContainerSpec{
				Limits: garden.Limits{
					Memory: garden.MemoryLimits{
						LimitInBytes: 30 * mb,
					},
				},
				Image: garden.ImageRef{
					URI: "docker://cfgarden/oom",
				},
			})
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			Expect(client.Destroy(container.Handle())).To(Succeed())
		})

		It("continues to receive OOM events", func() {
			stdout := gbytes.NewBuffer()
			stderr := gbytes.NewBuffer()
			process, err := container.Run(garden.ProcessSpec{
				Path: "/usemem",
			}, garden.ProcessIO{
				Stdout: stdout,
				Stderr: stderr,
			})
			Expect(err).NotTo(HaveOccurred())

			statusCode, err := process.Wait()
			Expect(err).NotTo(HaveOccurred())
			expectedMemoryCgroupPath := client.CgroupSubsystemPath("memory", container.Handle())
			Eventually(getEventsForContainer(container), time.Minute).Should(
				ContainElement("Out of memory"),
				fmt.Sprintf("Container PID: %s\nExpected memory cgroup path: %s\nPids in the container memory cgroup: %s",
					getContainerPid(container.Handle()),
					expectedMemoryCgroupPath,
					listPidsInCgroup(expectedMemoryCgroupPath),
				),
				fmt.Sprintf("%#v", map[string]string{
					"Status code":                          strconv.Itoa(statusCode),
					"Stdout":                               string(stdout.Contents()),
					"Stderr":                               string(stderr.Contents()),
					"Container PID":                        getContainerPid(container.Handle()),
					"Expected memory cgroup path":          expectedMemoryCgroupPath,
					"Pids in the container memory cgroup":  listPidsInCgroup(expectedMemoryCgroupPath),
					"Memory limit as listed in the cgroup": readFileString(filepath.Join(expectedMemoryCgroupPath, "memory.limit_in_bytes")),
					"Expected limit":                       strconv.FormatUint(30*mb, 10),
					"OOM Control":                          readFileString(filepath.Join(expectedMemoryCgroupPath, "memory.oom_control")),
				}),
				"<requesting dmesg>",
			)
		})

		It("does not leak containerd client sockets", func() {
			restartContainerd(client)
			Eventually(
				func() string {
					out, err := exec.Command("/bin/bash", "-c", fmt.Sprintf("ps aux | grep -E '[c]ontainerd-shim.*%s' | awk '{print $2}' | xargs -I {} sh -c 'ss -xp | grep {} | wc -l'", container.Handle())).CombinedOutput()
					Expect(err).NotTo(HaveOccurred())
					return strings.TrimSpace(string(out))
				}).Should(Equal("1"))
		})
	})
})

func listContainers(ctr, socket string) string {
	return runCtr(ctr, socket, []string{"containers", "list"})
}

func listTasks(ctr, socket string) string {
	return runCtr(ctr, socket, []string{"tasks", "list"})
}

func listProcesses(ctr, socket, containerID string) string {
	return runCtr(ctr, socket, []string{"tasks", "ps", containerID})
}

func pidFromProcessesOutput(processesOutput, id string) string {
	// processesOutput expected to be of the form:
	// PID      INFO
	// 23296    -
	// 23437    &ProcessDetails{ExecID:ctrd-pea-id,}

	processesOutputLines := strings.Split(processesOutput, "\n")

	for _, processesOutputLine := range processesOutputLines {
		if strings.Contains(processesOutputLine, id) {
			return strings.Split(processesOutputLine, " ")[0]
		}
	}

	return "0"
}

func getContainerPids(ctr, socket, containerID string) []string {
	// processesOutput expected to be of the form:
	// PID      INFO
	// 23296    -
	// 23437    &ProcessDetails{ExecID:ctrd-pea-id,}

	processesOutput := runCtr(ctr, socket, []string{"tasks", "ps", containerID})
	processesOutputLines := strings.Split(strings.TrimSpace(processesOutput), "\n")

	var pids []string
	// Here we exclude the first header line
	for _, processesOutputLine := range processesOutputLines[1:] {
		pids = append(pids, strings.Split(processesOutputLine, " ")[0])
	}

	return pids
}

func getContainerdProcessPid(ctr, socket, containerID, processID string) string {
	processesOutput := runCtr(ctr, socket, []string{"tasks", "ps", containerID})
	return pidFromProcessesOutput(processesOutput, processID)
}

func runCtr(ctr, socket string, args []string) string {
	defaultArgs := []string{"--address", socket, "--namespace", "garden"}
	cmd := exec.Command(ctr, append(defaultArgs, args...)...)

	output, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("ctr failed: %s", string(output)))

	return string(output)
}

func findFilesContaining(substring string) bool {
	filenames, _ := exec.Command("/usr/bin/find", "/", "-name", fmt.Sprintf("*%s*", substring)).Output()
	if strings.Contains(string(filenames), substring) {
		info := fmt.Sprintf("\nOutput of 'find': \n%s\nLooking for %s\n\n", filenames, substring)
		GinkgoWriter.Write([]byte(info))
		return true
	}

	return false
}

func psFaux() string {
	output, err := exec.Command("ps", "faux").Output()
	Expect(err).NotTo(HaveOccurred())
	return string(output)
}

func getParentPid(pid string) string {
	output, err := exec.Command("ps", "-o", "ppid=", "-p", pid).Output()
	Expect(err).NotTo(HaveOccurred())
	return strings.TrimSpace(string(output))
}

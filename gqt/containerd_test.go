package gqt_test

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

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
				It("creates a containerd container with a running task", func() {
					process, err := container.Run(garden.ProcessSpec{
						ID:    "ctrd-pea-id",
						Image: garden.ImageRef{URI: createPeaRootfsTar()},
						Path:  "/bin/sleep",
						Args:  []string{"10"},
						User:  "alice",
					}, garden.ProcessIO{})
					Expect(err).NotTo(HaveOccurred())

					containers := listContainers("ctr", config.ContainerdSocket)
					Expect(containers).To(ContainSubstring("ctrd-pea-id"))

					processes := listProcesses("ctr", config.ContainerdSocket, "ctrd-pea-id")
					Expect(processes).To(ContainSubstring("ctrd-pea-id"))

					peaProcessPid := pidFromProcessesOutput(processes, "ctrd-pea-id")
					cmdline := readFileString(filepath.Join("/", "proc", peaProcessPid, "cmdline"))
					Expect(cmdline).To(ContainSubstring("/bin/sleep"))

					code, err := process.Wait()
					Expect(err).NotTo(HaveOccurred())
					Expect(code).To(Equal(0))
				})

				It("cleans up pea-debris", func() {
					process, err := container.Run(garden.ProcessSpec{
						ID:    "ctrd-pea-id-2",
						Image: garden.ImageRef{URI: createPeaRootfsTar()},
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
						Image: garden.ImageRef{URI: createPeaRootfsTar()},
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
							Image: garden.ImageRef{URI: createPeaRootfsTar()},
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
							Image: garden.ImageRef{URI: createPeaRootfsTar()},
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
							Image: garden.ImageRef{URI: createPeaRootfsTar()},
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
							Image: garden.ImageRef{URI: createPeaRootfsTar()},
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

						Eventually(status).Should(Receive(BeEquivalentTo(42)))
					})
				})
			})
		})
	})

	Describe("containerd restart", func() {
		var container garden.Container

		JustBeforeEach(func() {
			var err error
			restartContainerd()
			container, err = client.Create(garden.ContainerSpec{Limits: garden.Limits{Memory: garden.MemoryLimits{LimitInBytes: 30 * mb}}})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(client.Destroy(container.Handle())).To(Succeed())
		})

		It("continues to receive OOM events", func() {
			process, err := container.Run(garden.ProcessSpec{
				Path: "dd",
				Args: []string{"if=/dev/urandom", "of=/dev/shm/foo", "bs=1M", "count=32"},
			}, garden.ProcessIO{})
			Expect(err).NotTo(HaveOccurred())

			_, err = process.Wait()
			Expect(err).NotTo(HaveOccurred())
			Eventually(getEventsForContainer(container), time.Minute).Should(ContainElement("Out of memory"))
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

func runCtr(ctr, socket string, args []string) string {
	defaultArgs := []string{"--address", socket, "--namespace", "garden"}
	cmd := exec.Command(ctr, append(defaultArgs, args...)...)

	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(session).Should(gexec.Exit(0), string(session.Err.Contents()))

	return string(session.Out.Contents())
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

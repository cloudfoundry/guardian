package nerd_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	apievents "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/nerd"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("Nerd", func() {
	var (
		testLogger    lager.Logger
		cnerd         *nerd.Nerd
		containerID   string
		processID     string
		fifoDir       string
		initProcessIO func() (io.Reader, io.Writer, io.Writer)
		processIO     func() (io.Reader, io.Writer, io.Writer, bool)
		stdin         io.Reader
		stdout        io.Writer
		stderr        io.Writer
		spec          *specs.Spec
	)

	const (
		maximusUID = 5000
		maximusGID = 5000
	)

	BeforeEach(func() {
		rand.Seed(time.Now().UnixNano())
		containerID = fmt.Sprintf("test-container-%s", randomString(10))
		processID = fmt.Sprintf("test-process-%s", randomString(10))
		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()
		stdin = nil
		initProcessIO = func() (io.Reader, io.Writer, io.Writer) {
			return stdin, stdout, stderr
		}
		processIO = func() (io.Reader, io.Writer, io.Writer, bool) {
			return stdin, stdout, stderr, false
		}
		testLogger = lagertest.NewTestLogger("nerd-test")

		var err error
		fifoDir, err = ioutil.TempDir("", "nerd-fifo")
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		cnerd = nerd.New(containerdClient, containerdContext, fifoDir)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(fifoDir)).To(Succeed())
		if spec != nil && spec.Root != nil {
			Expect(os.RemoveAll(filepath.Dir(spec.Root.Path))).To(Succeed())
		}
	})

	Describe("Create", func() {
		AfterEach(func() {
			cnerd.Delete(testLogger, containerID)
		})

		It("creates the containerd container by id", func() {
			spec = generateSpec(containerdContext, containerdClient, containerID)
			Expect(cnerd.Create(testLogger, containerID, spec, maximusUID, maximusGID, initProcessIO)).To(Succeed())

			containers := listContainers(testConfig.CtrBin, testConfig.Socket)
			Expect(containers).To(ContainSubstring(containerID))
		})

		It("starts an init process in the container", func() {
			spec = generateSpec(containerdContext, containerdClient, containerID)
			Expect(cnerd.Create(testLogger, containerID, spec, maximusUID, maximusGID, initProcessIO)).To(Succeed())

			containers := listProcesses(testConfig.CtrBin, testConfig.Socket, containerID)
			Expect(containers).To(ContainSubstring(containerID))
		})

		It("adds the annotations as labels", func() {
			spec = generateSpec(containerdContext, containerdClient, containerID)
			spec.Annotations = map[string]string{"hello": "world", "goodbye": "potato"}

			Expect(cnerd.Create(testLogger, containerID, spec, maximusUID, maximusGID, initProcessIO)).To(Succeed())
			Expect(containerLabels(testConfig.CtrBin, testConfig.Socket, containerID)).To(HaveKeyWithValue("hello", "world"))
			Expect(containerLabels(testConfig.CtrBin, testConfig.Socket, containerID)).To(HaveKeyWithValue("goodbye", "potato"))
		})

		It("writes stdout", func() {
			spec = generateSpec(containerdContext, containerdClient, containerID)
			spec.Process.Args = []string{"/bin/sh", "-c", "echo hi && sleep 60"}
			Expect(cnerd.Create(testLogger, containerID, spec, maximusUID, maximusGID, initProcessIO)).To(Succeed())
			Eventually(stdout, "30s").Should(gbytes.Say("hi"))
		})

		It("writes stderr", func() {
			spec = generateSpec(containerdContext, containerdClient, containerID)
			spec.Process.Args = []string{"/bin/sh", "-c", "echo hi 1>&2 && sleep 60"}
			Expect(cnerd.Create(testLogger, containerID, spec, maximusUID, maximusGID, initProcessIO)).To(Succeed())
			Eventually(stderr, "30s").Should(gbytes.Say("hi"))
		})

		It("reads from stdin", func() {
			stdin = bytes.NewBufferString("hi")
			spec = generateSpec(containerdContext, containerdClient, containerID)
			spec.Process.Args = []string{"/bin/sh", "-c", "cat && sleep 60"}
			Expect(cnerd.Create(testLogger, containerID, spec, maximusUID, maximusGID, initProcessIO)).To(Succeed())
			Eventually(stdout, "30s").Should(gbytes.Say("hi"))
		})
	})

	Describe("Spec", func() {
		AfterEach(func() {
			cnerd.Delete(testLogger, containerID)
		})

		It("returns the container spec", func() {
			spec = generateSpec(containerdContext, containerdClient, containerID)
			Expect(cnerd.Create(testLogger, containerID, spec, maximusUID, maximusGID, initProcessIO)).To(Succeed())

			actualSpec, err := cnerd.Spec(testLogger, containerID)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSpec).To(Equal(spec))
		})

		It("fails when container id not found", func() {
			_, err := cnerd.Spec(testLogger, "notAContainerId")
			Expect(err).To(MatchError("container notAContainerId not found"))
		})
	})

	Describe("Exec", func() {
		JustBeforeEach(func() {
			spec = generateSpec(containerdContext, containerdClient, containerID)
			Expect(cnerd.Create(testLogger, containerID, spec, maximusUID, maximusGID, initProcessIO)).To(Succeed())
		})

		AfterEach(func() {
			cnerd.Delete(testLogger, containerID)
		})

		It("execs a process in the container", func() {
			processSpec := &specs.Process{
				Args: []string{"/bin/sleep", "30"},
				Cwd:  "/",
			}

			err := cnerd.Exec(testLogger, containerID, processID, processSpec, processIO)
			Expect(err).NotTo(HaveOccurred())

			containers := listProcesses(testConfig.CtrBin, testConfig.Socket, containerID)
			Expect(containers).To(ContainSubstring(containerID)) // init process
			Expect(containers).To(ContainSubstring(processID))   // execed process
		})

		Describe("process IO", func() {
			It("reads from stdin", func() {
				processSpec := &specs.Process{
					Args: []string{"/bin/cat", "/proc/self/fd/0"},
					Cwd:  "/",
				}

				stdout := gbytes.NewBuffer()
				processIO = func() (io.Reader, io.Writer, io.Writer, bool) {
					stdin := gbytes.BufferWithBytes([]byte("hello nerd"))
					stdout := io.MultiWriter(stdout, GinkgoWriter)

					return stdin, stdout, nil, false
				}

				err := cnerd.Exec(testLogger, containerID, processID, processSpec, processIO)
				Expect(err).NotTo(HaveOccurred())
				Eventually(stdout).Should(gbytes.Say("hello nerd"))
			})

			It("writes to stdout", func() {
				processSpec := &specs.Process{
					Args: []string{"/bin/echo", "hello nerd"},
					Cwd:  "/",
				}

				stdout := gbytes.NewBuffer()
				processIO = func() (io.Reader, io.Writer, io.Writer, bool) {
					stdout := io.MultiWriter(stdout, GinkgoWriter)
					return nil, stdout, nil, false
				}

				err := cnerd.Exec(testLogger, containerID, processID, processSpec, processIO)
				Expect(err).NotTo(HaveOccurred())
				Eventually(stdout).Should(gbytes.Say("hello nerd"))
			})

			It("writes to stderr", func() {
				processSpec := &specs.Process{
					Args: []string{"/bin/cat", "notafile.txt"},
					Cwd:  "/",
				}

				stderr := gbytes.NewBuffer()
				processIO = func() (io.Reader, io.Writer, io.Writer, bool) {
					stderr := io.MultiWriter(stderr, GinkgoWriter)
					return nil, nil, stderr, false
				}

				err := cnerd.Exec(testLogger, containerID, processID, processSpec, processIO)
				Expect(err).NotTo(HaveOccurred())
				Eventually(stderr).Should(gbytes.Say("No such file"))
			})

			It("has access to a pseudo terminal", func() {
				processSpec := &specs.Process{
					Args: []string{"/bin/test", "-t", "1"},
					Cwd:  "/",
				}

				processIO = func() (io.Reader, io.Writer, io.Writer, bool) {
					return nil, nil, nil, true
				}

				err := cnerd.Exec(testLogger, containerID, processID, processSpec, processIO)
				Expect(err).NotTo(HaveOccurred())

				proc, err := cnerd.GetProcess(testLogger, containerID, processID)
				Expect(err).NotTo(HaveOccurred())
				exitCode, err := proc.Wait()
				Expect(err).NotTo(HaveOccurred())
				Expect(exitCode).To(BeZero())
			})
		})
	})

	Describe("Wait", func() {
		JustBeforeEach(func() {
			spec = generateSpec(containerdContext, containerdClient, containerID)
			Expect(cnerd.Create(testLogger, containerID, spec, maximusUID, maximusGID, initProcessIO)).To(Succeed())

			processSpec := &specs.Process{
				Args: []string{"/bin/sh", "-c", "exit 17"},
				Cwd:  "/",
			}

			err := cnerd.Exec(testLogger, containerID, processID, processSpec, processIO)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			cnerd.Delete(testLogger, containerID)
		})

		It("succeeds", func() {
			proc, err := cnerd.GetProcess(testLogger, containerID, processID)
			Expect(err).NotTo(HaveOccurred())
			_, err = proc.Wait()
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the exit code", func() {
			proc, err := cnerd.GetProcess(testLogger, containerID, processID)
			Expect(err).NotTo(HaveOccurred())
			exitCode, err := proc.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(17))
		})

		It("allows you to call Wait more than once", func() {
			proc, err := cnerd.GetProcess(testLogger, containerID, processID)
			Expect(err).NotTo(HaveOccurred())
			_, err = proc.Wait()
			Expect(err).NotTo(HaveOccurred())

			_, err = proc.Wait()
			Expect(err).NotTo(HaveOccurred())
		})

	})

	Describe("Signal", func() {
		JustBeforeEach(func() {
			spec = generateSpec(containerdContext, containerdClient, containerID)
			Expect(cnerd.Create(testLogger, containerID, spec, maximusUID, maximusGID, initProcessIO)).To(Succeed())
			stdoutBuffer := gbytes.NewBuffer()
			processIO = func() (io.Reader, io.Writer, io.Writer, bool) {
				return nil, stdoutBuffer, nil, false
			}

			processSpec := &specs.Process{
				Args: []string{"/bin/sh", "-c", `
					trap 'exit 42' TERM

					while true; do
					  echo 'sleeping'
					  sleep 1
					done
				`},
				Cwd: "/",
			}

			err := cnerd.Exec(testLogger, containerID, processID, processSpec, processIO)
			Expect(err).NotTo(HaveOccurred())

			Eventually(stdoutBuffer).Should(gbytes.Say("sleeping"))
		})

		AfterEach(func() {
			cnerd.Delete(testLogger, containerID)
		})

		It("should forward signals to the process", func() {
			proc, err := cnerd.GetProcess(testLogger, containerID, processID)
			Expect(err).NotTo(HaveOccurred())

			Expect(proc.Signal(syscall.SIGTERM)).To(Succeed())

			status := make(chan int)
			go func() {
				exit, err := proc.Wait()
				Expect(err).NotTo(HaveOccurred())
				status <- exit
			}()

			Eventually(status, 5*time.Second).Should(Receive(BeEquivalentTo(42)))
		})

	})

	Describe("GetProcess", func() {
		JustBeforeEach(func() {
			spec = generateSpec(containerdContext, containerdClient, containerID)
			Expect(cnerd.Create(testLogger, containerID, spec, maximusUID, maximusGID, initProcessIO)).To(Succeed())

			processSpec := &specs.Process{
				Args: []string{"/bin/sleep", "30"},
				Cwd:  "/",
			}

			err := cnerd.Exec(testLogger, containerID, processID, processSpec, processIO)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			cnerd.Delete(testLogger, containerID)
		})

		It("execs a process in the container", func() {
			proc, err := cnerd.GetProcess(testLogger, containerID, processID)
			Expect(err).NotTo(HaveOccurred())

			Expect(proc.ID()).To(Equal(processID))
		})

		Context("when the container does not exist", func() {
			JustBeforeEach(func() {
				cnerd.Delete(testLogger, containerID)
			})

			It("fails", func() {
				_, err := cnerd.GetProcess(testLogger, containerID, processID)
				Expect(err).To(MatchError(ContainSubstring("not found")))
			})
		})
	})

	Describe("GetTask", func() {

		JustBeforeEach(func() {
			spec = generateSpec(containerdContext, containerdClient, containerID)
			Expect(cnerd.Create(testLogger, containerID, spec, maximusUID, maximusGID, initProcessIO)).To(Succeed())
		})

		AfterEach(func() {
			cnerd.Delete(testLogger, containerID)
		})

		It("gets a backing process object containing the task for a given container ID", func() {
			task, err := cnerd.GetTask(testLogger, containerID)
			Expect(err).NotTo(HaveOccurred())
			Expect(task.ID()).To(Equal(containerID))
		})

		It("returns an error when the container doesn't exist", func() {
			_, err := cnerd.GetTask(testLogger, "does-not-exist")
			Expect(err).To(MatchError(runcontainerd.ContainerNotFoundError{Handle: "does-not-exist"}))
		})
	})

	Describe("RemoveBundle", func() {
		JustBeforeEach(func() {
			spec = generateSpec(containerdContext, containerdClient, containerID)
			Expect(cnerd.Create(testLogger, containerID, spec, maximusUID, maximusGID, initProcessIO)).To(Succeed())
			Expect(cnerd.Delete(testLogger, containerID)).To(Succeed())
		})

		It("deletes the containerd container by id", func() {
			Expect(cnerd.RemoveBundle(testLogger, containerID)).To(Succeed())

			containers := listContainers(testConfig.CtrBin, testConfig.Socket)
			Expect(containers).NotTo(ContainSubstring(containerID))
		})
	})

	Describe("Delete", func() {
		JustBeforeEach(func() {
			spec = generateSpec(containerdContext, containerdClient, containerID)
			Expect(cnerd.Create(testLogger, containerID, spec, maximusUID, maximusGID, initProcessIO)).To(Succeed())
		})

		It("deletes the containerd task by container id", func() {
			Expect(cnerd.Delete(testLogger, containerID)).To(Succeed())

			tasks := listTasks(testConfig.CtrBin, testConfig.Socket)
			Expect(tasks).NotTo(ContainSubstring(containerID))
		})
	})

	Describe("State", func() {
		JustBeforeEach(func() {
			spec = generateSpec(containerdContext, containerdClient, containerID)
			Expect(cnerd.Create(testLogger, containerID, spec, maximusUID, maximusGID, initProcessIO)).To(Succeed())
		})

		AfterEach(func() {
			cnerd.Delete(testLogger, containerID)
		})

		It("gets the pid and status of a running task", func() {
			pid, status, err := cnerd.State(testLogger, containerID)

			Expect(err).NotTo(HaveOccurred())
			Expect(pid).NotTo(BeZero())
			Expect(status).To(BeEquivalentTo(containerd.Running))
		})
	})

	Describe("GetContainerPID", func() {
		JustBeforeEach(func() {
			spec = generateSpec(containerdContext, containerdClient, containerID)
			Expect(cnerd.Create(testLogger, containerID, spec, maximusUID, maximusGID, initProcessIO)).To(Succeed())
		})

		AfterEach(func() {
			cnerd.Delete(testLogger, containerID)
		})

		It("gets the container init process pid", func() {
			procls := listProcesses(testConfig.CtrBin, testConfig.Socket, containerID)
			containerPid, err := cnerd.GetContainerPID(testLogger, containerID)
			Expect(err).ToNot(HaveOccurred())
			Expect(procls).To(ContainSubstring(strconv.Itoa(int(containerPid))))
		})
	})

	Describe("OOMEvents", func() {
		var (
			processSpec *specs.Process
			events      <-chan *apievents.TaskOOM
		)

		JustBeforeEach(func() {
			spec = generateSpec(containerdContext, containerdClient, containerID, withLinuxResources(specs.LinuxResources{
				Memory: &specs.LinuxMemory{
					Limit: int64ptr(30 * 1024 * 1024),
					Swap:  int64ptr(30 * 1024 * 1024),
				},
			}))
			Expect(cnerd.Create(testLogger, containerID, spec, maximusUID, maximusGID, initProcessIO)).To(Succeed())

			events = cnerd.OOMEvents(testLogger)

			err := cnerd.Exec(testLogger, containerID, processID, processSpec, processIO)
			Expect(err).NotTo(HaveOccurred())
			proc, err := cnerd.GetProcess(testLogger, containerID, processID)
			Expect(err).NotTo(HaveOccurred())
			_, err = proc.Wait()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			cnerd.Delete(testLogger, containerID)
		})

		Context("when OOM does not occur", func() {
			BeforeEach(func() {
				processSpec = &specs.Process{
					Args: []string{"/bin/echo", "hi"},
					Cwd:  "/",
				}
			})

			It("does not report OOM", func() {
				Consistently(events).ShouldNot(Receive())
			})
		})

		Context("when OOM occurs", func() {
			BeforeEach(func() {
				processSpec = &specs.Process{
					Args: []string{"/bin/usemem"},
					Cwd:  "/",
				}
			})

			It("reports OOM", func() {
				var event *apievents.TaskOOM
				Eventually(events, "10s").Should(Receive(&event), "<requesting dmesg>")
				Expect(event.ContainerID).To(Equal(containerID))
			})
		})
	})

	Context("when containerd is not running", func() {
		BeforeEach(func() {
			Expect(containerdProcess.Signal(syscall.SIGTERM)).To(Succeed())
			waitStatus, err := containerdProcess.Wait()
			Expect(err).NotTo(HaveOccurred())
			containerdProcess = nil
			Expect(waitStatus.ExitCode()).To(BeZero())
		})

		It("does not panic when loading the container metadata", func() {
			Expect(func() { cnerd.State(testLogger, "a-random-id") }).NotTo(Panic())
		})
	})

	Describe("BundleIDs", func() {
		AfterEach(func() {
			cnerd.Delete(testLogger, "banana")
			cnerd.Delete(testLogger, "banana2")
		})

		It("returns a filtered list of container handles", func() {
			spec = generateSpec(containerdContext, containerdClient, "banana")
			spec.Annotations = map[string]string{"hello": "potato", "abc": "boohoo"}
			Expect(cnerd.Create(testLogger, "banana", spec, maximusUID, maximusGID, initProcessIO)).To(Succeed())
			spec = generateSpec(containerdContext, containerdClient, "banana2")
			spec.Annotations = map[string]string{"abc": "boohoo"}
			Expect(cnerd.Create(testLogger, "banana2", spec, maximusUID, maximusGID, initProcessIO)).To(Succeed())

			handles, err := cnerd.BundleIDs(
				runcontainerd.ContainerFilter{
					Label:        "hello",
					Value:        "potato",
					ComparisonOp: "==",
				}, runcontainerd.ContainerFilter{
					Label:        "abc",
					Value:        "boohoo",
					ComparisonOp: "==",
				})

			Expect(err).NotTo(HaveOccurred())
			Expect(handles).To(ConsistOf("banana"))
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

func generateSpec(context context.Context, client *containerd.Client, containerID string, opts ...oci.SpecOpts) *specs.Spec {
	spec, err := oci.GenerateSpec(context, client, &containers.Container{ID: containerID}, opts...)
	Expect(err).NotTo(HaveOccurred())
	spec.Process.Args = []string{"/bin/sleep", "999999"}
	spec.Root = &specs.Root{
		Path: createRootfs(func(_ string) {}, 0755),
	}
	spec.Annotations = map[string]string{"container-type": "some-container-type"}

	return spec
}

func withLinuxResources(resources specs.LinuxResources) func(context.Context, oci.Client, *containers.Container, *oci.Spec) error {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, spec *oci.Spec) error {
		spec.Linux.Resources = &resources
		return nil
	}
}

func listContainers(ctr, socket string) string {
	return runCtr(ctr, socket, []string{"containers", "list"})
}

func listTasks(ctr, socket string) string {
	return runCtr(ctr, socket, []string{"tasks", "list"})
}

func listProcesses(ctr, socket, containerID string) string {
	return runCtr(ctr, socket, []string{"tasks", "ps", containerID})
}

func containerLabels(ctr, socket, handle string) map[string]interface{} {
	output := runCtr(ctr, socket, []string{"containers", "info", handle})

	var result map[string]interface{}
	Expect(json.Unmarshal([]byte(output), &result)).To(Succeed())
	return result["Labels"].(map[string]interface{})
}

func runCtr(ctr, socket string, args []string) string {
	defaultArgs := []string{"--address", socket, "--namespace", fmt.Sprintf("nerdspace%d", GinkgoParallelNode())}
	cmd := exec.Command(ctr, append(defaultArgs, args...)...)

	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(session, "10s").Should(gexec.Exit(0))

	return string(session.Out.Contents())
}

func randomInt(min, max int) int {
	return min + rand.Intn(max-min)
}

func randomString(len int) string {
	bytes := make([]byte, len)
	for i := 0; i < len; i++ {
		bytes[i] = byte(randomInt(65, 90))
	}
	return string(bytes)
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

func int64ptr(i int64) *int64 {
	return &i
}

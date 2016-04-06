package gqt_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"syscall"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/types"
)

var _ = Describe("Run", func() {
	var client *runner.RunningGarden

	AfterEach(func() {
		// Ensure we don't leak process.json files
		matches, err := filepath.Glob(fmt.Sprintf("%s/guardianprocess*", client.Tmpdir))
		Expect(err).ToNot(HaveOccurred())
		Expect(len(matches)).To(Equal(0))

		Expect(client.DestroyAndStop()).To(Succeed())
	})

	DescribeTable("running a process",
		func(spec garden.ProcessSpec, matchers ...func(actual interface{})) {
			client = startGarden()
			container, err := client.Create(garden.ContainerSpec{
				RootFSPath: runner.RootFSPath,
			})
			Expect(err).NotTo(HaveOccurred())

			out := gbytes.NewBuffer()
			proc, err := container.Run(
				spec,
				garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, out),
					Stderr: io.MultiWriter(GinkgoWriter, out),
				})
			Expect(err).NotTo(HaveOccurred())

			exitCode, err := proc.Wait()
			Expect(err).NotTo(HaveOccurred())

			for _, m := range matchers {
				m(&process{exitCode, out})
			}
		},

		Entry("with an absolute path",
			spec("/bin/sh", "-c", "echo hello; exit 12"),
			should(gbytes.Say("hello"), gexec.Exit(12)),
		),

		Entry("with a path to be found in a regular user's path",
			spec("sh", "-c", "echo potato; exit 24"),
			should(gbytes.Say("potato"), gexec.Exit(24)),
		),

		Entry("with a path that doesn't exist",
			spec("potato"),
			shouldNot(gexec.Exit(0)),
		),
	)

	Describe("security", func() {
		Describe("rlimits", func() {
			It("sets requested rlimits, even if they are increased above current limit", func() {
				var old syscall.Rlimit
				Expect(syscall.Getrlimit(syscall.RLIMIT_NOFILE, &old)).To(Succeed())

				Expect(syscall.Setrlimit(syscall.RLIMIT_NOFILE, &syscall.Rlimit{
					Max: 100000,
					Cur: 100000,
				})).To(Succeed())

				defer syscall.Setrlimit(syscall.RLIMIT_NOFILE, &old)

				client = startGarden()
				container, err := client.Create(garden.ContainerSpec{
					RootFSPath: runner.RootFSPath,
					Privileged: false,
				})
				Expect(err).NotTo(HaveOccurred())

				limit := uint64(100001)
				stdout := gbytes.NewBuffer()
				process, err := container.Run(garden.ProcessSpec{
					User: "root",
					Path: "/bin/sh",
					Args: []string{"-c", "ulimit -a"},
					Limits: garden.ResourceLimits{
						Nofile: &limit,
					},
				}, garden.ProcessIO{
					Stdout: stdout,
					Stderr: GinkgoWriter,
				})
				Expect(err).ToNot(HaveOccurred())

				Expect(process.Wait()).To(Equal(0))
				Expect(stdout).To(gbytes.Say("file descriptors\\W+100001"))
			})
		})

		Describe("symlinks", func() {
			var (
				target, rootfs string
			)

			BeforeEach(func() {
				var err error
				rootfs, err = ioutil.TempDir("", "symlinks")
				Expect(err).NotTo(HaveOccurred())

				Expect(os.Mkdir(filepath.Join(rootfs, "tmp"), 0777)).To(Succeed())

				target, err = ioutil.TempDir("", "symlinkstarget")
				Expect(err).NotTo(HaveOccurred())

				Expect(os.Symlink(target, path.Join(rootfs, "symlink"))).To(Succeed())
			})

			It("does not follow symlinks into the host when creating cwd", func() {
				client = startGarden()
				container, err := client.Create(garden.ContainerSpec{RootFSPath: rootfs})
				Expect(err).NotTo(HaveOccurred())

				process, err := container.Run(garden.ProcessSpec{
					Path: "echo",
					Args: []string{"hello"},
					Dir:  "/symlink/foo/bar",
				}, garden.ProcessIO{Stdout: GinkgoWriter, Stderr: GinkgoWriter})
				Expect(err).NotTo(HaveOccurred())

				Expect(process.Wait()).ToNot(Equal(0)) // `echo` wont exist in the fake rootfs. This is fine.
				Expect(path.Join(target, "foo")).NotTo(BeADirectory())
			})
		})
	})

	Context("when container is privileged", func() {
		It("can run a process as a particular user", func() {
			client = startGarden()
			container, err := client.Create(garden.ContainerSpec{
				RootFSPath: runner.RootFSPath,
				Privileged: true,
			})
			Expect(err).NotTo(HaveOccurred())

			out := gbytes.NewBuffer()
			proc, err := container.Run(
				garden.ProcessSpec{
					Path: "whoami",
					User: "alice",
				},
				garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, out),
					Stderr: io.MultiWriter(GinkgoWriter, out),
				})
			Expect(err).NotTo(HaveOccurred())

			exitCode, err := proc.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(0))

			Expect(out).To(gbytes.Say("alice"))
		})
	})

	Describe("PATH env variable", func() {
		var container garden.Container

		BeforeEach(func() {
			client = startGarden()
			var err error
			container, err = client.Create(garden.ContainerSpec{
				RootFSPath: runner.RootFSPath,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		DescribeTable("contains the correct default values", func(user, path string) {
			out := gbytes.NewBuffer()
			proc, err := container.Run(
				garden.ProcessSpec{
					Path: "sh",
					Args: []string{"-c", "echo $PATH"},
					User: user,
				},
				garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, out),
					Stderr: io.MultiWriter(GinkgoWriter, out),
				})
			Expect(err).NotTo(HaveOccurred())

			exitCode, err := proc.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(0))

			Expect(out).To(gbytes.Say(path))
		},
			Entry("for a non-root user", "alice", `^/usr/local/bin:/usr/bin:/bin\n$`),
			Entry("for the root user", "root",
				`^/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n$`),
		)
	})

	Describe("user env variable", func() {
		var container garden.Container

		BeforeEach(func() {
			client = startGarden()
			var err error
			container, err = client.Create(garden.ContainerSpec{
				RootFSPath: runner.RootFSPath,
				Env:        []string{"USER=ppp", "HOME=/home/ppp"},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		DescribeTable("contains the correct values", func(user string, env, paths []string) {
			out := gbytes.NewBuffer()
			proc, err := container.Run(
				garden.ProcessSpec{
					Path: "sh",
					Args: []string{"-c", "env"},
					User: user,
					Env:  env,
				},
				garden.ProcessIO{
					Stdout: io.MultiWriter(GinkgoWriter, out),
					Stderr: io.MultiWriter(GinkgoWriter, out),
				})
			Expect(err).NotTo(HaveOccurred())

			exitCode, err := proc.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(0))

			for _, path := range paths {
				Expect(out).To(gbytes.Say(path))
			}
		},
			Entry("for empty user", "", []string{}, []string{"USER=ppp", "HOME=/home/ppp"}),
			Entry("when we specify the env in processSpec", "alice", []string{"USER=alice", "HI=YO"}, []string{"USER=alice", "HOME=/home/ppp", "HI=YO"}),
		)
	})

	Describe("Signalling", func() {
		It("should forward SIGTERM to the process", func(done Done) {
			client = startGarden()

			container, err := client.Create(garden.ContainerSpec{
				RootFSPath: runner.RootFSPath,
			})
			Expect(err).NotTo(HaveOccurred())

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

			Expect(proc.Wait()).To(Equal(42))

			close(done)
		}, 20.0)
	})
})

func should(matchers ...types.GomegaMatcher) func(actual interface{}) {
	return func(actual interface{}) {
		for _, matcher := range matchers {
			Expect(actual).To(matcher)
		}
	}
}

func shouldNot(matchers ...types.GomegaMatcher) func(actual interface{}) {
	return func(actual interface{}) {
		for _, matcher := range matchers {
			Expect(actual).ToNot(matcher)
		}
	}
}

func spec(path string, args ...string) garden.ProcessSpec {
	return garden.ProcessSpec{
		Path: path,
		Args: args,
	}
}

type process struct {
	exitCode int
	buffer   *gbytes.Buffer
}

func (p *process) ExitCode() int {
	return p.exitCode
}

func (p *process) Buffer() *gbytes.Buffer {
	return p.buffer
}

package cgroups_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	. "code.cloudfoundry.org/commandrunner/fake_command_runner/matchers"
	"code.cloudfoundry.org/guardian/rundmc/cgroups"
	fakes "code.cloudfoundry.org/guardian/rundmc/cgroups/cgroupsfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CgroupStarter", func() {
	var (
		runner          *fake_command_runner.FakeCommandRunner
		starter         *cgroups.CgroupStarter
		procCgroups     *FakeReadCloser
		procSelfCgroups *FakeReadCloser
		logger          lager.Logger
		chowner         *fakes.FakeChowner

		tmpDir string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "gdncgroup")
		Expect(err).NotTo(HaveOccurred())

		logger = lagertest.NewTestLogger("test")
		runner = fake_command_runner.New()
		procCgroups = &FakeReadCloser{Buffer: bytes.NewBufferString("")}
		procSelfCgroups = &FakeReadCloser{Buffer: bytes.NewBufferString("")}
		chowner = &fakes.FakeChowner{}
		starter = &cgroups.CgroupStarter{
			CgroupPath:      path.Join(tmpDir, "cgroup"),
			GardenCgroup:    "garden",
			CommandRunner:   runner,
			ProcCgroups:     procCgroups,
			ProcSelfCgroups: procSelfCgroups,
			Logger:          logger,
			Chowner:         chowner,
		}
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	It("mkdirs the cgroup path", func() {
		starter.Start()
		Expect(path.Join(tmpDir, "cgroup")).To(BeADirectory())
	})

	Context("when the cgroup path is not a mountpoint", func() {
		BeforeEach(func() {
			runner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "mountpoint",
				Args: []string{"-q", path.Join(tmpDir, "cgroup") + "/"},
			}, func(cmd *exec.Cmd) error {
				return errors.New("not a mountpoint")
			})
		})

		It("mounts it", func() {
			starter.Start()
			Expect(runner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
				Path: "mount",
				Args: []string{"-t", "tmpfs", "-o", "uid=0,gid=0,mode=0755", "cgroup", path.Join(tmpDir, "cgroup")},
			}))
		})
	})

	Context("when the cgroup path exists", func() {
		It("does not mount it again", func() {
			starter.Start()
			Expect(runner).NotTo(HaveExecutedSerially(fake_command_runner.CommandSpec{
				Path: "mount",
				Args: []string{"-t", "tmpfs", "-o", "uid=0,gid=0,mode=0755", "cgroup", path.Join(tmpDir, "cgroup")},
			}))
		})
	})

	Context("with a sane /proc/cgroups and /proc/self/cgroup", func() {
		BeforeEach(func() {
			_, err := procCgroups.Write([]byte(
				"#subsys_name\thierarchy\tnum_cgroups\tenabled\n" +
					"devices\t1\t1\t1\n" +
					"memory\t2\t1\t1\n" +
					"cpu\t3\t1\t1\n" +
					"cpuacct\t4\t1\t1\n",
			))
			Expect(err).NotTo(HaveOccurred())

			_, err = procSelfCgroups.Write([]byte(
				"5:devices:/\n" +
					"4:memory:/\n" +
					"3:cpu,cpuacct:/\n",
			))
			Expect(err).NotTo(HaveOccurred())

			for _, notMounted := range []string{"devices", "cpu", "cpuacct"} {
				runner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "mountpoint",
					Args: []string{"-q", path.Join(tmpDir, "cgroup", notMounted) + "/"},
				}, func(cmd *exec.Cmd) error {
					return errors.New("not a mountpoint")
				})
			}
		})

		It("succeeds", func() {
			Expect(starter.Start()).To(Succeed())
		})

		It("mounts the hierarchies which are not already mounted", func() {
			starter.Start()

			Expect(runner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
				Path: "mount",
				Args: []string{"-n", "-t", "cgroup", "-o", "devices", "cgroup", path.Join(tmpDir, "cgroup", "devices")},
			}))

			Expect(runner).NotTo(HaveExecutedSerially(fake_command_runner.CommandSpec{
				Path: "mount",
				Args: []string{"-n", "-t", "cgroup", "-o", "memory", "cgroup", path.Join(tmpDir, "cgroup", "memory")},
			}))

			Expect(runner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
				Path: "mount",
				Args: []string{"-n", "-t", "cgroup", "-o", "cpu,cpuacct", "cgroup", path.Join(tmpDir, "cgroup", "cpu")},
			}))

			Expect(runner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
				Path: "mount",
				Args: []string{"-n", "-t", "cgroup", "-o", "cpu,cpuacct", "cgroup", path.Join(tmpDir, "cgroup", "cpuacct")},
			}))
		})

		It("creates needed directories", func() {
			starter.Start()
			Expect(path.Join(tmpDir, "cgroup", "devices")).To(BeADirectory())
		})

		It("creates subdirectories owned by the specified user and group", func() {
			starter.Start()
			allChowns := []string{}
			for i := 0; i < chowner.RecursiveChownCallCount(); i++ {
				allChowns = append(allChowns, chowner.RecursiveChownArgsForCall(i))
			}

			for _, subsystem := range []string{"devices", "cpu", "memory"} {
				fullPath := path.Join(tmpDir, "cgroup", subsystem, "garden")
				Expect(fullPath).To(BeADirectory())
				Expect(allChowns).To(ContainElement(fullPath))
				dirStat, err := os.Stat(fullPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(dirStat.Mode() & os.ModePerm).To(Equal(os.FileMode(0700)))
			}
		})

		Context("when we are in the nested case", func() {
			BeforeEach(func() {
				_, err := procCgroups.Write([]byte(
					"#subsys_name\thierarchy\tnum_cgroups\tenabled\n" +
						"memory\t2\t1\t1\n",
				))
				Expect(err).NotTo(HaveOccurred())

				_, err = procSelfCgroups.Write([]byte(
					"4:memory:/461299e6-b672-497c-64e5-793494b9bbdb\n",
				))
				Expect(err).NotTo(HaveOccurred())

				for _, notMounted := range []string{"memory"} {
					runner.WhenRunning(fake_command_runner.CommandSpec{
						Path: "mountpoint",
						Args: []string{"-q", path.Join(tmpDir, "cgroup", notMounted) + "/"},
					}, func(cmd *exec.Cmd) error {
						return errors.New("not a mountpoint")
					})
				}
			})

			It("creates subdirectories owned by the specified user and group", func() {
				starter.Start()
				allChowns := []string{}
				for i := 0; i < chowner.RecursiveChownCallCount(); i++ {
					allChowns = append(allChowns, chowner.RecursiveChownArgsForCall(i))
				}

				for _, subsystem := range []string{"memory"} {
					fullPath := path.Join(tmpDir, "cgroup", subsystem, "461299e6-b672-497c-64e5-793494b9bbdb", "garden")
					Expect(fullPath).To(BeADirectory())
					Expect(allChowns).To(ContainElement(fullPath))
					dirStat, err := os.Stat(fullPath)
					Expect(err).NotTo(HaveOccurred())
					Expect(dirStat.Mode() & os.ModePerm).To(Equal(os.FileMode(0700)))
				}
			})
		})

		Context("when a subsystem is not yet mounted anywhere", func() {
			BeforeEach(func() {
				_, err := procCgroups.Write([]byte("freezer\t7\t1\t1\n"))
				Expect(err).NotTo(HaveOccurred())

				runner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "mountpoint",
					Args: []string{"-q", path.Join(tmpDir, "cgroup", "freezer") + "/"},
				}, func(cmd *exec.Cmd) error {
					return errors.New("not a mountpoint")
				})
			})

			It("mounts it as its own subsystem", func() {
				starter.Start()

				Expect(runner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
					Path: "mount",
					Args: []string{"-n", "-t", "cgroup", "-o", "freezer", "cgroup", path.Join(tmpDir, "cgroup", "freezer")},
				}))
			})
		})

		Context("when a subsystem is disabled", func() {
			BeforeEach(func() {
				_, err := procCgroups.Write([]byte("freezer\t7\t1\t0\n"))
				Expect(err).NotTo(HaveOccurred())

				runner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "mountpoint",
					Args: []string{"-q", path.Join(tmpDir, "cgroup", "freezer") + "/"},
				}, func(cmd *exec.Cmd) error {
					return errors.New("not a mountpoint")
				})
			})

			It("skips it", func() {
				starter.Start()

				Expect(runner).ToNot(HaveExecutedSerially(fake_command_runner.CommandSpec{
					Path: "mount",
					Args: []string{"-n", "-t", "cgroup", "-o", "freezer", "cgroup", path.Join(tmpDir, "cgroup", "freezer")},
				}))
			})
		})
	})

	Context("when /proc/cgroups contains malformed entries", func() {
		BeforeEach(func() {
			_, err := procCgroups.Write([]byte(
				"#subsys_name\thierarchy\tnum_cgroups\tenabled\n" +
					"devices\tA ONE AND A\t1\t1\n" +
					"memory\tTWO AND A\t1\t1\n" +
					"cpu\tTHREE AND A\t1\t1\n" +
					"cpuacct\tFOUR\t1\t1\n",
			))
			Expect(err).NotTo(HaveOccurred())

			_, err = procSelfCgroups.Write([]byte(
				"5:devices:/\n" +
					"4:memory:/\n" +
					"3:cpu,cpuacct:/\n",
			))
			Expect(err).NotTo(HaveOccurred())

			for _, notMounted := range []string{"devices", "cpu", "cpuacct"} {
				runner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "mountpoint",
					Args: []string{"-q", path.Join(tmpDir, "cgroup", notMounted) + "/"},
				}, func(cmd *exec.Cmd) error {
					return errors.New("not a mountpoint")
				})
			}
		})

		It("returns CgroupsFormatError", func() {
			err := starter.Start()
			Expect(err).To(Equal(cgroups.CgroupsFormatError{Content: "devices\tA ONE AND A\t1\t1"}))
		})
	})

	Context("when /proc/cgroups is empty", func() {
		BeforeEach(func() {
			_, err := procCgroups.Write([]byte(""))
			Expect(err).NotTo(HaveOccurred())

			_, err = procSelfCgroups.Write([]byte(
				"5:devices:/\n" +
					"4:memory:/\n" +
					"3:cpu,cpuacct:/\n",
			))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns CgroupsFormatError", func() {
			err := starter.Start()
			Expect(err).To(Equal(cgroups.CgroupsFormatError{Content: "(empty)"}))
		})
	})

	Context("when /proc/cgroups contains an unknown header scheme", func() {
		BeforeEach(func() {
			_, err := procCgroups.Write([]byte(
				"#subsys_name\tsome\tbogus\tcolumns\n" +
					"devices\t1\t1\t1" +
					"memory\t2\t1\t1" +
					"cpu\t3\t1\t1" +
					"cpuacct\t4\t1\t1",
			))
			Expect(err).NotTo(HaveOccurred())

			_, err = procSelfCgroups.Write([]byte(
				"5:devices:/\n" +
					"4:memory:/\n" +
					"3:cpu,cpuacct:/\n",
			))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns CgroupsFormatError", func() {
			err := starter.Start()
			Expect(err).To(Equal(cgroups.CgroupsFormatError{Content: "#subsys_name\tsome\tbogus\tcolumns"}))
		})
	})

	It("closes the procCgroups reader", func() {
		starter.Start()
		Expect(procCgroups.closed).To(BeTrue())
	})

	It("closes the procSelfCgroups reader", func() {
		starter.Start()
		Expect(procSelfCgroups.closed).To(BeTrue())
	})
})

type FakeReadCloser struct {
	closed bool
	*bytes.Buffer
}

func (f *FakeReadCloser) Close() error {
	f.closed = true
	return nil
}

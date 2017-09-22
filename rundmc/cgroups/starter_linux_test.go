package cgroups_test

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	. "code.cloudfoundry.org/commandrunner/fake_command_runner/matchers"
	"code.cloudfoundry.org/guardian/rundmc/cgroups"
	fakes "code.cloudfoundry.org/guardian/rundmc/cgroups/cgroupsfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("CgroupStarter", func() {
	var (
		runner                  *fake_command_runner.FakeCommandRunner
		starter                 *cgroups.CgroupStarter
		logger                  lager.Logger
		chowner                 *fakes.FakeChowner
		procCgroupsContents     string
		procSelfCgroupsContents string

		tmpDir string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "gdncgroup")
		Expect(err).NotTo(HaveOccurred())

		procSelfCgroupsContents = ""
		procCgroupsContents = "#subsys_name\thierarchy\tnum_cgroups\tenabled\n" +
			"devices\t1\t1\t1\n"

		logger = lagertest.NewTestLogger("test")
		runner = fake_command_runner.New()
		chowner = new(fakes.FakeChowner)
	})

	JustBeforeEach(func() {
		starter = &cgroups.CgroupStarter{
			CgroupPath:   path.Join(tmpDir, "cgroup"),
			GardenCgroup: "garden",
			AllowedDevices: []specs.LinuxDeviceCgroup{{
				Type:   "c",
				Major:  int64ptr(10),
				Minor:  int64ptr(200),
				Access: "rwm",
			}},
			CommandRunner:   runner,
			ProcCgroups:     ioutil.NopCloser(strings.NewReader(procCgroupsContents)),
			ProcSelfCgroups: ioutil.NopCloser(strings.NewReader(procSelfCgroupsContents)),
			Logger:          logger,
			Chowner:         chowner,
		}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	It("mkdirs the cgroup path", func() {
		starter.Start()
		Expect(path.Join(tmpDir, "cgroup")).To(BeADirectory())
	})

	It("adds the right content into devices.allow", func() {
		Expect(starter.Start()).To(Succeed())

		Expect(path.Join(tmpDir, "cgroup", "devices", "garden")).To(BeADirectory())

		content := readFile(path.Join(tmpDir, "cgroup", "devices", "garden", "devices.allow"))
		Expect(string(content)).To(Equal("c 10:200 rwm"))
	})

	It("adds the right content into devices.deny", func() {
		Expect(starter.Start()).To(Succeed())

		Expect(path.Join(tmpDir, "cgroup", "devices", "garden")).To(BeADirectory())

		content := readFile(path.Join(tmpDir, "cgroup", "devices", "garden", "devices.deny"))
		Expect(string(content)).To(Equal("a"))
	})

	Context("when there is already a child device cgroup", func() {
		JustBeforeEach(func() {
			Expect(os.MkdirAll(path.Join(tmpDir, "cgroup", "devices", "garden", "child"), 0777)).To(Succeed())
		})

		It("does not write to devices.deny", func() {
			Expect(starter.Start()).To(Succeed())
			Expect(path.Join(tmpDir, "cgroup", "devices", "garden")).To(BeADirectory())
			Expect(path.Join(tmpDir, "cgroup", "devices", "garden", "devices.deny")).NotTo(BeAnExistingFile())
		})

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
			procCgroupsContents = "#subsys_name\thierarchy\tnum_cgroups\tenabled\n" +
				"devices\t1\t1\t1\n" +
				"memory\t2\t1\t1\n" +
				"cpu\t3\t1\t1\n" +
				"cpuacct\t4\t1\t1\n"

			procSelfCgroupsContents = "5:devices:/\n" +
				"4:memory:/\n" +
				"3:cpu,cpuacct:/\n"

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
				Expect(dirStat.Mode() & os.ModePerm).To(Equal(os.FileMode(0755)))
			}
		})

		Context("when the garden folder already exists", func() {
			BeforeEach(func() {
				for _, subsystem := range []string{"devices", "cpu", "memory"} {
					fullPath := path.Join(tmpDir, "cgroup", subsystem, "garden")
					Expect(fullPath).ToNot(BeADirectory())
					Expect(os.MkdirAll(fullPath, 0700))
				}
			})

			It("changes the permissions of the subdirectories", func() {
				starter.Start()
				for _, subsystem := range []string{"devices", "cpu", "memory"} {
					fullPath := path.Join(tmpDir, "cgroup", subsystem, "garden")
					dirStat, err := os.Stat(fullPath)
					Expect(err).NotTo(HaveOccurred())
					Expect(dirStat.Mode() & os.ModePerm).To(Equal(os.FileMode(0755)))
				}
			})
		})

		Context("when we are in the nested case", func() {
			BeforeEach(func() {
				procCgroupsContents = "#subsys_name\thierarchy\tnum_cgroups\tenabled\n" +
					"memory\t2\t1\t1\n"

				procSelfCgroupsContents = "4:memory:/461299e6-b672-497c-64e5-793494b9bbdb\n"

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
					Expect(dirStat.Mode() & os.ModePerm).To(Equal(os.FileMode(0755)))
				}
			})
		})

		Context("when a subsystem is not yet mounted anywhere", func() {
			BeforeEach(func() {
				procCgroupsContents = "#subsys_name\thierarchy\tnum_cgroups\tenabled\n" +
					"freezer\t7\t1\t1\n"

				runner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "mountpoint",
					Args: []string{"-q", path.Join(tmpDir, "cgroup", "freezer") + "/"},
				}, func(cmd *exec.Cmd) error {
					return errors.New("not a mountpoint")
				})
			})

			It("mounts it as its own subsystem", func() {
				Expect(starter.Start()).To(Succeed())
				Expect(runner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
					Path: "mount",
					Args: []string{"-n", "-t", "cgroup", "-o", "freezer", "cgroup", path.Join(tmpDir, "cgroup", "freezer")},
				}))
			})
		})

		Context("when a subsystem is disabled", func() {
			BeforeEach(func() {
				procCgroupsContents = "#subsys_name\thierarchy\tnum_cgroups\tenabled\n" +
					"freezer\t7\t1\t0\n"

				runner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "mountpoint",
					Args: []string{"-q", path.Join(tmpDir, "cgroup", "freezer") + "/"},
				}, func(cmd *exec.Cmd) error {
					return errors.New("not a mountpoint")
				})
			})

			It("skips it", func() {
				Expect(starter.Start()).To(Succeed())

				Expect(runner).ToNot(HaveExecutedSerially(fake_command_runner.CommandSpec{
					Path: "mount",
					Args: []string{"-n", "-t", "cgroup", "-o", "freezer", "cgroup", path.Join(tmpDir, "cgroup", "freezer")},
				}))
			})
		})
	})

	Context("when /proc/cgroups contains malformed entries", func() {
		BeforeEach(func() {
			procCgroupsContents = "#subsys_name\thierarchy\tnum_cgroups\tenabled\n" +
				"devices\tA ONE AND A\t1\t1\n" +
				"memory\tTWO AND A\t1\t1\n" +
				"cpu\tTHREE AND A\t1\t1\n" +
				"cpuacct\tFOUR\t1\t1\n"

			procSelfCgroupsContents = "5:devices:/\n" +
				"4:memory:/\n" +
				"3:cpu,cpuacct:/\n"

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
			procCgroupsContents = ""

			procSelfCgroupsContents = "5:devices:/\n" +
				"4:memory:/\n" +
				"3:cpu,cpuacct:/\n"
		})

		It("returns CgroupsFormatError", func() {
			err := starter.Start()
			Expect(err).To(Equal(cgroups.CgroupsFormatError{Content: "(empty)"}))
		})
	})

	Context("when /proc/cgroups contains an unknown header scheme", func() {
		BeforeEach(func() {
			procCgroupsContents = "#subsys_name\tsome\tbogus\tcolumns\n" +
				"devices\t1\t1\t1" +
				"memory\t2\t1\t1" +
				"cpu\t3\t1\t1" +
				"cpuacct\t4\t1\t1"

			procSelfCgroupsContents = "5:devices:/\n" +
				"4:memory:/\n" +
				"3:cpu,cpuacct:/\n"
		})

		It("returns CgroupsFormatError", func() {
			err := starter.Start()
			Expect(err).To(Equal(cgroups.CgroupsFormatError{Content: "#subsys_name\tsome\tbogus\tcolumns"}))
		})
	})
})

func readFile(path string) []byte {
	content, err := ioutil.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	return content
}

func int64ptr(i int64) *int64 {
	return &i
}

package cgroups_test

import (
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	gardencgroups "code.cloudfoundry.org/guardian/rundmc/cgroups"
	"code.cloudfoundry.org/guardian/rundmc/cgroups/fs/fsfakes"
	"code.cloudfoundry.org/guardian/rundmc/rundmcfakes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

var _ = Describe("cgroupstarter", func() {
	var (
		starter *gardencgroups.CgroupStarter

		logger                    lager.Logger
		mountPointChecker         *rundmcfakes.FakeMountPointChecker
		fakeFS                    *fsfakes.FakeFS
		procCgroupsContents       string
		procSelfCgroupsContents   string
		cgroupPathMounted         bool
		cgroupPathMountCheckError error
		notMountedCgroups         []string
		cpuThrottlingEnabled      bool

		tmpDir     string
		mountPoint string
	)

	BeforeEach(func() {
		tmpDir = tempDir("", "gdncgroup")

		procSelfCgroupsContents = ""
		procCgroupsContents = "#subsys_name\thierarchy\tnum_cgroups\tenabled\n" +
			"devices\t1\t1\t1\n"

		logger = lagertest.NewTestLogger("test")
		fakeFS = new(fsfakes.FakeFS)
		mountPointChecker = new(rundmcfakes.FakeMountPointChecker)
		cgroupPathMounted = true
		cgroupPathMountCheckError = nil
		notMountedCgroups = []string{}
		cpuThrottlingEnabled = false
		mountPoint = path.Join(tmpDir, "cgroup")
	})

	JustBeforeEach(func() {
		mountPointChecker.Stub = func(p string) (bool, error) {
			for _, notMounted := range notMountedCgroups {
				if p == path.Join(tmpDir, "cgroup", notMounted) {
					return false, nil
				}
			}

			if p == path.Join(tmpDir, "cgroup") {
				return cgroupPathMounted, cgroupPathMountCheckError
			}

			return true, nil
		}

		starter = gardencgroups.NewStarter(
			logger,
			io.NopCloser(strings.NewReader(procCgroupsContents)),
			io.NopCloser(strings.NewReader(procSelfCgroupsContents)),
			mountPoint,
			"garden",
			[]specs.LinuxDeviceCgroup{{
				Type:   "c",
				Major:  int64ptr(1),
				Minor:  int64ptr(7),
				Access: "rwm",
			}},
			mountPointChecker.Spy,
			cpuThrottlingEnabled,
		)
		starter.FS = fakeFS
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	Context("cgroups v2", func() {
		BeforeEach(func() {
			if !cgroups.IsCgroup2UnifiedMode() {
				Skip("Skipping cgroups v2 tests when cgroups v1 is enabled")
			}
		})

		It("mkdirs the cgroup path", func() {
			starter.Start()
			Expect(path.Join(tmpDir, "cgroup")).To(BeADirectory())
		})

		Context("when the cgroup path is not a mountpoint", func() {
			BeforeEach(func() {
				cgroupPathMounted = false
			})

			It("mounts it", func() {
				starter.Start()

				Expect(fakeFS.MountCallCount()).To(BeNumerically(">", 0))
				expected := newMountArgs("cgroup", filepath.Join(tmpDir, "cgroup"), "tmpfs", 0, "uid=0,gid=0,mode=0755")
				Expect(newMountArgs(fakeFS.MountArgsForCall(0))).To(Equal(expected))
			})
		})

		Context("when the cgroup path is a mountpoint", func() {
			It("does not mount it again", func() {
				starter.Start()
				for i := 0; i < fakeFS.MountCallCount(); i++ {
					Expect(newMountArgs(fakeFS.MountArgsForCall(i)).target).NotTo(Equal(filepath.Join(tmpDir, "cgroup")))
				}
			})
		})

		Context("when there is an error checking for a mountpoint on Start", func() {
			BeforeEach(func() {
				cgroupPathMountCheckError = errors.New("mountpoint check error")
			})

			It("returns an error", func() {
				Expect(starter.Start()).To(MatchError("mountpoint check error"))
			})
		})

		It("mounts the unified hierarchy", func() {
			starter.Start()

			Expect(fakeFS.MountCallCount()).To(Equal(1))

			expected := newMountArgs("cgroup", filepath.Join(tmpDir, "cgroup", "unified"), "cgroup2", 0, "")
			Expect(newMountArgs(fakeFS.MountArgsForCall(0))).To(Equal(expected))
		})

		Context("when mount point is a unified mountpoint", func() {
			BeforeEach(func() {
				mountPoint = "/sys/fs/cgroup"
			})

			It("does not mount the unified hierarchy", func() {
				starter.Start()

				Expect(fakeFS.MountCallCount()).To(Equal(0))
			})
		})
	})

	Context("cgroups v1", func() {
		BeforeEach(func() {
			if cgroups.IsCgroup2UnifiedMode() {
				Skip("Skipping cgroups v1 tests when cgroups v2 is enabled")
			}
		})

		It("mkdirs the cgroup path", func() {
			starter.Start()
			Expect(path.Join(tmpDir, "cgroup")).To(BeADirectory())
		})

		It("adds the right content into devices.allow", func() {
			Expect(starter.Start()).To(Succeed())

			Expect(path.Join(tmpDir, "cgroup", "devices", "garden")).To(BeADirectory())

			content := readFile(path.Join(tmpDir, "cgroup", "devices", "garden", "devices.allow"))
			Expect(string(content)).To(Equal("c 1:7 rwm"))
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
				cgroupPathMounted = false
			})

			It("mounts it", func() {
				Expect(starter.Start()).To(Succeed())

				Expect(fakeFS.MountCallCount()).To(BeNumerically(">", 0))
				expected := newMountArgs("cgroup", filepath.Join(tmpDir, "cgroup"), "tmpfs", 0, "uid=0,gid=0,mode=0755")
				Expect(newMountArgs(fakeFS.MountArgsForCall(0))).To(Equal(expected))
			})
		})

		Context("when the cgroup path exists", func() {
			It("does not mount it again", func() {
				Expect(starter.Start()).To(Succeed())
				for i := 0; i < fakeFS.MountCallCount(); i++ {
					Expect(newMountArgs(fakeFS.MountArgsForCall(i)).target).NotTo(Equal(filepath.Join(tmpDir, "cgroup")))
				}
			})
		})

		Context("when there is an error checking for a mountpoint on Start", func() {
			BeforeEach(func() {
				cgroupPathMountCheckError = errors.New("mountpoint check error")
			})

			It("returns an error", func() {
				Expect(starter.Start()).To(MatchError("mountpoint check error"))
			})
		})

		Context("with a sane /proc/gardencgroups and /proc/self/cgroup", func() {
			BeforeEach(func() {
				procCgroupsContents = "#subsys_name\thierarchy\tnum_cgroups\tenabled\n" +
					"devices\t1\t1\t1\n" +
					"memory\t2\t1\t1\n" +
					"cpu\t3\t1\t1\n" +
					"cpuacct\t4\t1\t1\n"

				procSelfCgroupsContents = "5:devices:/\n" +
					"4:memory:/\n" +
					"3:cpu,cpuacct:/\n"

				notMountedCgroups = []string{"devices", "cpu", "cpuacct"}
			})

			It("succeeds", func() {
				Expect(starter.Start()).To(Succeed())
			})

			It("mounts the hierarchies which are not already mounted", func() {
				Expect(starter.Start()).To(Succeed())

				Expect(fakeFS.MountCallCount()).To(Equal(4))

				expected := newMountArgs("cgroup", filepath.Join(tmpDir, "cgroup", "devices"), "cgroup", 0, "devices")
				Expect(newMountArgs(fakeFS.MountArgsForCall(0))).To(Equal(expected))

				expected = newMountArgs("cgroup", filepath.Join(tmpDir, "cgroup", "memory"), "cgroup", 0, "memory")
				Expect(newMountArgs(fakeFS.MountArgsForCall(1))).To(Equal(expected))

				expected = newMountArgs("cgroup", filepath.Join(tmpDir, "cgroup", "cpu"), "cgroup", 0, "cpu,cpuacct")
				Expect(newMountArgs(fakeFS.MountArgsForCall(2))).To(Equal(expected))

				expected = newMountArgs("cgroup", filepath.Join(tmpDir, "cgroup", "cpuacct"), "cgroup", 0, "cpu,cpuacct")
				Expect(newMountArgs(fakeFS.MountArgsForCall(3))).To(Equal(expected))
			})

			It("creates needed directories", func() {
				starter.Start()
				Expect(path.Join(tmpDir, "cgroup", "devices")).To(BeADirectory())
			})

			It("creates subdirectories owned by the specified user and group", func() {
				Expect(starter.WithUID(123).WithGID(987).Start()).To(Succeed())
				allChowns := []string{}
				for i := 0; i < fakeFS.ChownCallCount(); i++ {
					path, uid, gid := fakeFS.ChownArgsForCall(i)
					allChowns = append(allChowns, path)
					Expect(uid).To(Equal(123))
					Expect(gid).To(Equal(987))
				}

				for _, subsystem := range []string{"devices", "cpu", "memory"} {
					fullPath := path.Join(tmpDir, "cgroup", subsystem, "garden")
					Expect(fullPath).To(BeADirectory())
					Expect(allChowns).To(ContainElement(fullPath))
					Expect(stat(fullPath).Mode() & os.ModePerm).To(Equal(os.FileMode(0755)))
				}
			})

			Context("when the garden folder already exists", func() {
				BeforeEach(func() {
					for _, subsystem := range []string{"devices", "cpu", "memory"} {
						fullPath := path.Join(tmpDir, "cgroup", subsystem, "garden")
						Expect(fullPath).ToNot(BeADirectory())
						Expect(os.MkdirAll(fullPath, 0700)).To(Succeed())
					}
				})

				It("changes the permissions of the subdirectories", func() {
					starter.Start()
					for _, subsystem := range []string{"devices", "cpu", "memory"} {
						fullPath := path.Join(tmpDir, "cgroup", subsystem, "garden")
						Expect(stat(fullPath).Mode() & os.ModePerm).To(Equal(os.FileMode(0755)))
					}
				})
			})

			Context("when we are in the nested case", func() {
				BeforeEach(func() {
					procCgroupsContents = "#subsys_name\thierarchy\tnum_cgroups\tenabled\n" +
						"memory\t2\t1\t1\n"

					procSelfCgroupsContents = "4:memory:/461299e6-b672-497c-64e5-793494b9bbdb\n"
					notMountedCgroups = []string{"memory"}
				})

				It("creates subdirectories owned by the specified user and group", func() {
					Expect(starter.WithUID(123).WithGID(987).Start()).To(Succeed())
					allChowns := []string{}
					for i := 0; i < fakeFS.ChownCallCount(); i++ {
						path, uid, gid := fakeFS.ChownArgsForCall(i)
						Expect(uid).To(Equal(123))
						Expect(gid).To(Equal(987))
						allChowns = append(allChowns, path)
					}

					for _, subsystem := range []string{"memory"} {
						fullPath := path.Join(tmpDir, "cgroup", subsystem, "461299e6-b672-497c-64e5-793494b9bbdb", "garden")
						Expect(fullPath).To(BeADirectory())
						Expect(allChowns).To(ContainElement(fullPath))
						Expect(stat(fullPath).Mode() & os.ModePerm).To(Equal(os.FileMode(0755)))
					}
				})
			})

			Context("when a subsystem is not yet mounted anywhere", func() {
				BeforeEach(func() {
					procCgroupsContents = "#subsys_name\thierarchy\tnum_cgroups\tenabled\n" +
						"freezer\t7\t1\t1\n"
					notMountedCgroups = []string{"freezer"}
				})

				It("mounts it as its own subsystem", func() {
					Expect(starter.Start()).To(Succeed())
					Expect(fakeFS.MountCallCount()).To(Equal(1))
					expected := newMountArgs("cgroup", filepath.Join(tmpDir, "cgroup", "freezer"), "cgroup", 0, "freezer")
					Expect(newMountArgs(fakeFS.MountArgsForCall(0))).To(Equal(expected))
				})
			})

			Context("when a subsystem is disabled", func() {
				BeforeEach(func() {
					procCgroupsContents = "#subsys_name\thierarchy\tnum_cgroups\tenabled\n" +
						"freezer\t7\t1\t0\n"
					notMountedCgroups = []string{"freezer"}
				})

				It("skips it", func() {
					Expect(starter.Start()).To(Succeed())
					Expect(fakeFS.MountCallCount()).To(Equal(0))
				})
			})

			Context("when /proc/self/cgroup contains named cgroup hierarchies", func() {
				BeforeEach(func() {
					procSelfCgroupsContents = procSelfCgroupsContents + "1:name=systemd:/\n"
				})

				Context("when the named cgroup is already mounted", func() {
					BeforeEach(func() {
						notMountedCgroups = []string{}
					})

					It("does not mount it again", func() {
						Expect(fakeFS.MountCallCount()).To(Equal(0))
					})
				})

				Context("when the named cgroup is not mounted", func() {
					BeforeEach(func() {
						notMountedCgroups = []string{"systemd"}
					})

					It("mounts it with name option as its own subsystem", func() {
						Expect(starter.Start()).To(Succeed())
						Expect(fakeFS.MountCallCount()).To(BeNumerically(">", 0))
						var mountArgs []mountArgs
						for i := 0; i < fakeFS.MountCallCount(); i++ {
							mountArgs = append(mountArgs, newMountArgs(fakeFS.MountArgsForCall(i)))
						}
						Expect(mountArgs).To(ContainElement(newMountArgs("cgroup", filepath.Join(tmpDir, "cgroup", "systemd"), "cgroup", 0, "name=systemd")))
					})
				})
			})

			Context("when a cgroup is already mounted", func() {
				BeforeEach(func() {
					fakeFS.MountReturns(unix.EBUSY)
				})

				It("succeeds", func() {
					Expect(starter.Start()).To(Succeed())
				})
			})
		})

		Context("when /proc/gardencgroups contains malformed entries", func() {
			BeforeEach(func() {
				procCgroupsContents = "#subsys_name\thierarchy\tnum_cgroups\tenabled\n" +
					"devices\tA ONE AND A\t1\t1\n" +
					"memory\tTWO AND A\t1\t1\n" +
					"cpu\tTHREE AND A\t1\t1\n" +
					"cpuacct\tFOUR\t1\t1\n"

				procSelfCgroupsContents = "5:devices:/\n" +
					"4:memory:/\n" +
					"3:cpu,cpuacct:/\n"
				notMountedCgroups = []string{"devices", "cpu", "cpuacct"}
			})

			It("returns gardencgroupsFormatError", func() {
				err := starter.Start()
				Expect(err).To(Equal(gardencgroups.CgroupsFormatError{Content: "devices\tA ONE AND A\t1\t1"}))
			})
		})

		Context("when /proc/gardencgroups is empty", func() {
			BeforeEach(func() {
				procCgroupsContents = ""

				procSelfCgroupsContents = "5:devices:/\n" +
					"4:memory:/\n" +
					"3:cpu,cpuacct:/\n"
			})

			It("returns gardencgroupsFormatError", func() {
				err := starter.Start()
				Expect(err).To(Equal(gardencgroups.CgroupsFormatError{Content: "(empty)"}))
			})
		})

		Context("when /proc/gardencgroups contains an unknown header scheme", func() {
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

			It("returns gardencgroupsFormatError", func() {
				err := starter.Start()
				Expect(err).To(Equal(gardencgroups.CgroupsFormatError{Content: "#subsys_name\tsome\tbogus\tcolumns"}))
			})
		})

		Context("when cpu throttling is enabled", func() {
			BeforeEach(func() {
				procCgroupsContents = "#subsys_name\thierarchy\tnum_cgroups\tenabled\n" +
					"devices\t1\t1\t1\n" +
					"memory\t2\t1\t1\n" +
					"cpu\t3\t1\t1\n" +
					"cpuacct\t4\t1\t1\n"

				procSelfCgroupsContents = "5:devices:/\n" +
					"4:memory:/\n" +
					"3:cpu,cpuacct:/\n"

				notMountedCgroups = []string{"devices", "cpu", "cpuacct"}
				cpuThrottlingEnabled = true
			})

			It("adds the right content into devices.allow", func() {
				Expect(starter.Start()).To(Succeed())

				Expect(path.Join(tmpDir, "cgroup", "devices", "garden", gardencgroups.GoodCgroupName)).To(BeADirectory())

				content := readFile(path.Join(tmpDir, "cgroup", "devices", "garden", gardencgroups.GoodCgroupName, "devices.allow"))
				Expect(string(content)).To(Equal("c 1:7 rwm"))
			})

			It("adds the right content into devices.deny", func() {
				Expect(starter.Start()).To(Succeed())

				Expect(path.Join(tmpDir, "cgroup", "devices", "garden", gardencgroups.GoodCgroupName)).To(BeADirectory())

				content := readFile(path.Join(tmpDir, "cgroup", "devices", "garden", gardencgroups.GoodCgroupName, "devices.deny"))
				Expect(string(content)).To(Equal("a"))
			})

			It("creates subdirectories owned by the specified user and group", func() {
				Expect(starter.WithUID(123).WithGID(987).Start()).To(Succeed())
				allChowns := []string{}
				for i := 0; i < fakeFS.ChownCallCount(); i++ {
					path, uid, gid := fakeFS.ChownArgsForCall(i)
					allChowns = append(allChowns, path)
					Expect(uid).To(Equal(123))
					Expect(gid).To(Equal(987))
				}

				for _, subsystem := range []string{"devices", "cpu", "memory"} {
					fullPath := path.Join(tmpDir, "cgroup", subsystem, "garden")
					Expect(fullPath).To(BeADirectory())
					Expect(allChowns).To(ContainElement(fullPath))
					Expect(stat(fullPath).Mode() & os.ModePerm).To(Equal(os.FileMode(0755)))
				}
			})

			It("does not create a bad cgroup for other subsystems", func() {
				Expect(starter.WithUID(123).WithGID(987).Start()).To(Succeed())
				for _, subsystem := range []string{"devices", "memory"} {
					fullPath := path.Join(tmpDir, "cgroup", subsystem, "garden", gardencgroups.BadCgroupName)
					Expect(fullPath).ToNot(BeADirectory())
				}
			})

			It("creates the bad CPU group owned by the specified user and group", func() {
				Expect(starter.WithUID(123).WithGID(987).Start()).To(Succeed())
				allChowns := []string{}
				for i := 0; i < fakeFS.ChownCallCount(); i++ {
					path, uid, gid := fakeFS.ChownArgsForCall(i)
					allChowns = append(allChowns, path)
					Expect(uid).To(Equal(123))
					Expect(gid).To(Equal(987))
				}

				fullPath := path.Join(tmpDir, "cgroup", "cpu", "garden", gardencgroups.BadCgroupName)
				Expect(fullPath).To(BeADirectory())
				Expect(allChowns).To(ContainElement(fullPath))
				Expect(stat(fullPath).Mode() & os.ModePerm).To(Equal(os.FileMode(0755)))
			})

			Context("when the garden folder already exists", func() {
				BeforeEach(func() {
					for _, subsystem := range []string{"devices", "cpu", "memory"} {
						fullPath := path.Join(tmpDir, "cgroup", subsystem, "garden", gardencgroups.GoodCgroupName)
						Expect(fullPath).ToNot(BeADirectory())
						Expect(os.MkdirAll(fullPath, 0700)).To(Succeed())
					}
				})

				It("changes the permissions of the subdirectories", func() {
					starter.Start()
					for _, subsystem := range []string{"devices", "cpu", "memory"} {
						fullPath := path.Join(tmpDir, "cgroup", subsystem, "garden", gardencgroups.GoodCgroupName)
						Expect(stat(fullPath).Mode() & os.ModePerm).To(Equal(os.FileMode(0755)))
					}
				})
			})
		})
	})
})

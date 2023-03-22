package processes_test

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/processes"
	fakes "code.cloudfoundry.org/guardian/rundmc/processes/processesfakes"
	"code.cloudfoundry.org/guardian/rundmc/users"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("ProcBuilder", func() {
	var (
		envDeterminer  *fakes.FakeEnvDeterminer
		isRootless     bool
		nonRootMaxCaps = []string{"CAP_FOO", "CAP_BAZ"}

		procBuilder *processes.ProcBuilder

		bndl        goci.Bndl
		processSpec garden.ProcessSpec
		user        *users.ExecUser
	)

	BeforeEach(func() {
		envDeterminer = new(fakes.FakeEnvDeterminer)
		envDeterminer.EnvForReturns([]string{"ENV"})
		isRootless = false

		bndl = goci.Bundle().
			WithHostname("some-hostname").
			WithUIDMappings(specs.LinuxIDMapping{
				ContainerID: 0,
				HostID:      10,
				Size:        100,
			}).
			WithGIDMappings(specs.LinuxIDMapping{
				ContainerID: 0,
				HostID:      20,
				Size:        100,
			}).
			WithProcess(specs.Process{
				ApparmorProfile: "default-profile",
			}).
			WithCapabilities("CAP_FOO", "CAP_BAR")
		processSpec = garden.ProcessSpec{
			Path: "program",
			Args: []string{"and", "args"},
			User: "Froderick",
			Dir:  "dir",
			Limits: garden.ResourceLimits{
				As:         ptr(12),
				Core:       ptr(24),
				Cpu:        ptr(36),
				Data:       ptr(99),
				Fsize:      ptr(101),
				Locks:      ptr(111),
				Memlock:    ptr(987),
				Msgqueue:   ptr(777),
				Nice:       ptr(111),
				Nofile:     ptr(222),
				Nproc:      ptr(1234),
				Rss:        ptr(888),
				Rtprio:     ptr(254),
				Sigpending: ptr(101),
				Stack:      ptr(44),
			},
		}
		user = &users.ExecUser{
			Uid:   1,
			Gid:   2,
			Sgids: []int{5, 6, 7},
		}
	})

	Describe("the built process", func() {
		var preparedProc *specs.Process

		JustBeforeEach(func() {
			procBuilder = processes.NewBuilder(envDeterminer, isRootless, nonRootMaxCaps)
			preparedProc = procBuilder.BuildProcess(bndl, processSpec, user)
		})

		It("merges the path and args to create the argv", func() {
			Expect(preparedProc.Args).To(Equal([]string{"program", "and", "args"}))
		})

		Context("when the bundle has no mappings for host root (container is privileged)", func() {
			BeforeEach(func() {
				bndl.Spec.Linux.UIDMappings = nil
				bndl.Spec.Linux.GIDMappings = nil
			})

			It("sets the rlimits correctly", func() {
				Expect(preparedProc.Rlimits).To(ConsistOf(
					specs.POSIXRlimit{Type: "RLIMIT_AS", Hard: 12, Soft: 12},
					specs.POSIXRlimit{Type: "RLIMIT_CORE", Hard: 24, Soft: 24},
					specs.POSIXRlimit{Type: "RLIMIT_CPU", Hard: 36, Soft: 36},
					specs.POSIXRlimit{Type: "RLIMIT_DATA", Hard: 99, Soft: 99},
					specs.POSIXRlimit{Type: "RLIMIT_FSIZE", Hard: 101, Soft: 101},
					specs.POSIXRlimit{Type: "RLIMIT_LOCKS", Hard: 111, Soft: 111},
					specs.POSIXRlimit{Type: "RLIMIT_MEMLOCK", Hard: 987, Soft: 987},
					specs.POSIXRlimit{Type: "RLIMIT_MSGQUEUE", Hard: 777, Soft: 777},
					specs.POSIXRlimit{Type: "RLIMIT_NICE", Hard: 111, Soft: 111},
					specs.POSIXRlimit{Type: "RLIMIT_NOFILE", Hard: 222, Soft: 222},
					specs.POSIXRlimit{Type: "RLIMIT_NPROC", Hard: 1234, Soft: 1234},
					specs.POSIXRlimit{Type: "RLIMIT_RSS", Hard: 888, Soft: 888},
					specs.POSIXRlimit{Type: "RLIMIT_RTPRIO", Hard: 254, Soft: 254},
					specs.POSIXRlimit{Type: "RLIMIT_SIGPENDING", Hard: 101, Soft: 101},
					specs.POSIXRlimit{Type: "RLIMIT_STACK", Hard: 44, Soft: 44},
				))
			})

			Context("when no TTY is specified", func() {
				It("sets terminal to false", func() {
					Expect(preparedProc.Terminal).To(BeFalse())
				})

				It("sets a default console size anyway, which will not be used", func() {
					Expect(preparedProc.ConsoleSize.Height).To(BeEquivalentTo(24))
					Expect(preparedProc.ConsoleSize.Width).To(BeEquivalentTo(80))
				})
			})

			Context("when a TTY is specified", func() {
				BeforeEach(func() {
					processSpec.TTY = &garden.TTYSpec{
						WindowSize: &garden.WindowSize{
							Columns: 25,
							Rows:    81,
						},
					}
				})

				It("sets the console size", func() {
					Expect(preparedProc.ConsoleSize.Width).To(BeEquivalentTo(25))
					Expect(preparedProc.ConsoleSize.Height).To(BeEquivalentTo(81))
				})

				It("sets terminal to false", func() {
					Expect(preparedProc.Terminal).To(BeTrue())
				})

				Context("when the user is root", func() {
					BeforeEach(func() {
						user.Uid = 0
						user.Gid = 0
					})

					It("passes the specified capabilities", func() {
						caps := []string{"CAP_FOO", "CAP_BAR"}
						Expect(preparedProc.Capabilities.Effective).To(BeEmpty())
						Expect(preparedProc.Capabilities.Bounding).To(Equal(caps))
						Expect(preparedProc.Capabilities.Inheritable).To(Equal(caps))
						Expect(preparedProc.Capabilities.Permitted).To(Equal(caps))
						Expect(preparedProc.Capabilities.Ambient).To(BeEmpty())
					})
				})

				Context("when the user is not root", func() {
					It("passes the intersection of the specified capabilites and the non-root max capabilities", func() {
						caps := []string{"CAP_FOO"}
						Expect(preparedProc.Capabilities.Effective).To(BeEmpty())
						Expect(preparedProc.Capabilities.Bounding).To(Equal(caps))
						Expect(preparedProc.Capabilities.Inheritable).To(Equal(caps))
						Expect(preparedProc.Capabilities.Permitted).To(Equal(caps))
						Expect(preparedProc.Capabilities.Ambient).To(BeEmpty())
					})
				})

				Context("when no capabilities are specified", func() {
					BeforeEach(func() {
						bndl.Spec.Process.Capabilities = nil
					})

					It("does not set the capabilities object on the process", func() {
						Expect(preparedProc.Capabilities).To(BeNil())
					})
				})

				It("passes the AppArmor profile", func() {
					Expect(preparedProc.ApparmorProfile).To(Equal("default-profile"))
				})

				It("passes the UID, GID and SGIDs", func() {
					Expect(preparedProc.User.UID).To(Equal(uint32(1)))
					Expect(preparedProc.User.GID).To(Equal(uint32(2)))
					Expect(preparedProc.User.AdditionalGids).To(Equal([]uint32{5, 6, 7}))
				})

				When("garden is running as rootless", func() {
					BeforeEach(func() {
						isRootless = true
					})

					It("passes an empty supplementary group list", func() {
						Expect(preparedProc.User.AdditionalGids).To(BeEmpty())
					})
				})

				It("passes the username, which is used on Windows", func() {
					Expect(preparedProc.User.Username).To(Equal("Froderick"))
				})

				It("passes the working dir", func() {
					Expect(preparedProc.Cwd).To(Equal("dir"))
				})

				It("passes the environment from the EnvDeterminer", func() {
					Expect(envDeterminer.EnvForCallCount()).To(Equal(1))
					actualBndl, actualSpec, actualContainerUID := envDeterminer.EnvForArgsForCall(0)
					Expect(actualBndl).To(Equal(bndl))
					Expect(actualSpec).To(Equal(processSpec))
					Expect(actualContainerUID).To(Equal(user.Uid))
					Expect(preparedProc.Env).To(ConsistOf("ENV"))
				})
			})
		})
	})
})

func ptr(i uint64) *uint64 {
	return &i
}

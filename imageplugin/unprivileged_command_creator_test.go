package imageplugin_test

import (
	"net/url"
	"os/exec"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden-shed/rootfs_provider"
	"code.cloudfoundry.org/guardian/imageplugin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("UnprivilegedCommandCreator", func() {
	var (
		commandCreator *imageplugin.UnprivilegedCommandCreator
		binPath        string
		extraArgs      []string
		idMappings     []specs.LinuxIDMapping
	)

	BeforeEach(func() {
		binPath = "/image-plugin"
		extraArgs = []string{}
		idMappings = []specs.LinuxIDMapping{
			specs.LinuxIDMapping{
				ContainerID: 0,
				HostID:      100,
				Size:        1,
			},
			specs.LinuxIDMapping{
				ContainerID: 1,
				HostID:      1,
				Size:        99,
			},
		}
	})

	JustBeforeEach(func() {
		commandCreator = &imageplugin.UnprivilegedCommandCreator{
			BinPath:    binPath,
			ExtraArgs:  extraArgs,
			IDMappings: idMappings,
		}
	})

	Describe("CreateCommand", func() {
		var (
			createCmd *exec.Cmd
			spec      rootfs_provider.Spec
		)

		BeforeEach(func() {
			rootfsURL, err := url.Parse("/fake-registry/image")
			Expect(err).NotTo(HaveOccurred())
			spec = rootfs_provider.Spec{RootFS: rootfsURL}
		})

		JustBeforeEach(func() {
			var err error
			createCmd, err = commandCreator.CreateCommand(nil, "test-handle", spec)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns a command with the correct image plugin path", func() {
			Expect(createCmd.Path).To(Equal(binPath))
		})

		It("returns a command with the create action", func() {
			Expect(createCmd.Args[1]).To(Equal("create"))
		})

		It("returns a command that has the id mappings as args", func() {
			Expect(createCmd.Args).To(HaveLen(12))
			Expect(createCmd.Args[2]).To(Equal("--uid-mapping"))
			Expect(createCmd.Args[3]).To(Equal("0:100:1"))
			Expect(createCmd.Args[4]).To(Equal("--gid-mapping"))
			Expect(createCmd.Args[5]).To(Equal("0:100:1"))
			Expect(createCmd.Args[6]).To(Equal("--uid-mapping"))
			Expect(createCmd.Args[7]).To(Equal("1:1:99"))
			Expect(createCmd.Args[8]).To(Equal("--gid-mapping"))
			Expect(createCmd.Args[9]).To(Equal("1:1:99"))
		})

		It("returns a command with the provided rootfs as image", func() {
			Expect(createCmd.Args[10]).To(Equal("/fake-registry/image"))
		})

		It("returns a command with the provided handle as id", func() {
			Expect(createCmd.Args[11]).To(Equal("test-handle"))
		})

		Context("when using a docker image", func() {
			BeforeEach(func() {
				var err error
				spec.RootFS, err = url.Parse("docker:///busybox#1.26.1")
				Expect(err).NotTo(HaveOccurred())
			})

			It("replaces the '#' with ':'", func() {
				Expect(createCmd.Args[10]).To(Equal("docker:///busybox:1.26.1"))
			})
		})

		Context("when disk quota is provided", func() {
			Context("and the quota size is = 0", func() {
				BeforeEach(func() {
					spec.QuotaSize = 0
				})

				It("returns a command without the quota", func() {
					Expect(createCmd.Args).NotTo(ContainElement("--disk-limit-size-bytes"))
				})
			})

			Context("and the quota size is > 0", func() {
				BeforeEach(func() {
					spec.QuotaSize = 100000
				})

				It("returns a command with the quota", func() {
					Expect(createCmd.Args[10]).To(Equal("--disk-limit-size-bytes"))
					Expect(createCmd.Args[11]).To(Equal("100000"))
				})

				Context("and it's got an exclusive scope", func() {
					BeforeEach(func() {
						spec.QuotaScope = garden.DiskLimitScopeExclusive
					})

					It("returns a command with the quota and an exclusive scope", func() {
						Expect(createCmd.Args[10]).To(Equal("--disk-limit-size-bytes"))
						Expect(createCmd.Args[11]).To(Equal("100000"))

						Expect(createCmd.Args).To(ContainElement("--exclude-image-from-quota"))
					})
				})

				Context("and it's got a total scope", func() {
					BeforeEach(func() {
						spec.QuotaScope = garden.DiskLimitScopeTotal
					})

					It("returns a command with the quota and a total scope", func() {
						Expect(createCmd.Args[10]).To(Equal("--disk-limit-size-bytes"))
						Expect(createCmd.Args[11]).To(Equal("100000"))

						Expect(createCmd.Args).NotTo(ContainElement("--exclude-image-from-quota"))
					})
				})
			})
		})

		Context("when extra args are provided", func() {
			BeforeEach(func() {
				extraArgs = []string{"foo", "bar"}
			})

			It("returns a command with the extra args as global args preceeding the action", func() {
				Expect(createCmd.Args[1]).To(Equal("foo"))
				Expect(createCmd.Args[2]).To(Equal("bar"))
				Expect(createCmd.Args[3]).To(Equal("create"))
			})
		})

		It("returns a command that runs as an unprivileged user", func() {
			Expect(createCmd.SysProcAttr.Credential.Uid).To(Equal(idMappings[0].HostID))
			Expect(createCmd.SysProcAttr.Credential.Gid).To(Equal(idMappings[0].HostID))
		})
	})

	Describe("DestroyCommand", func() {
		var (
			destroyCmd *exec.Cmd
		)

		JustBeforeEach(func() {
			destroyCmd = commandCreator.DestroyCommand(nil, "test-handle")
		})

		It("returns a command with the correct image plugin path", func() {
			Expect(destroyCmd.Path).To(Equal(binPath))
		})

		It("returns a command with the delete action", func() {
			Expect(destroyCmd.Args[1]).To(Equal("delete"))
		})

		It("returns a command with the provided handle as id", func() {
			Expect(destroyCmd.Args[2]).To(Equal("test-handle"))
		})

		Context("when extra args are provided", func() {
			BeforeEach(func() {
				extraArgs = []string{"foo", "bar"}
			})

			It("returns a command with the extra args as global args preceeding the action", func() {
				Expect(destroyCmd.Args[1]).To(Equal("foo"))
				Expect(destroyCmd.Args[2]).To(Equal("bar"))
				Expect(destroyCmd.Args[3]).To(Equal("delete"))
			})
		})

		It("returns a command that runs as an unprivileged user", func() {
			Expect(destroyCmd.SysProcAttr.Credential.Uid).To(Equal(idMappings[0].HostID))
			Expect(destroyCmd.SysProcAttr.Credential.Gid).To(Equal(idMappings[0].HostID))
		})
	})

	Describe("MetricsCommand", func() {
		var (
			metricsCmd *exec.Cmd
		)

		JustBeforeEach(func() {
			metricsCmd = commandCreator.MetricsCommand(nil, "test-handle")
		})

		It("returns a command with the correct image plugin path", func() {
			Expect(metricsCmd.Path).To(Equal(binPath))
		})

		It("returns a command with the stats action", func() {
			Expect(metricsCmd.Args[1]).To(Equal("stats"))
		})

		It("returns a command with the provided handle as id", func() {
			Expect(metricsCmd.Args[2]).To(Equal("test-handle"))
		})

		Context("when extra args are provided", func() {
			BeforeEach(func() {
				extraArgs = []string{"foo", "bar"}
			})

			It("returns a command with the extra args as global args preceeding the action", func() {
				Expect(metricsCmd.Args[1]).To(Equal("foo"))
				Expect(metricsCmd.Args[2]).To(Equal("bar"))
				Expect(metricsCmd.Args[3]).To(Equal("stats"))
			})
		})

		It("returns a command that runs as an unprivileged user", func() {
			Expect(metricsCmd.SysProcAttr.Credential.Uid).To(Equal(idMappings[0].HostID))
			Expect(metricsCmd.SysProcAttr.Credential.Gid).To(Equal(idMappings[0].HostID))
		})
	})
})

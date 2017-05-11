package imageplugin_test

import (
	"net/url"
	"os/exec"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden-shed/rootfs_spec"
	"code.cloudfoundry.org/guardian/imageplugin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DefaultCommandCreator", func() {
	var (
		commandCreator *imageplugin.DefaultCommandCreator
		binPath        string
		extraArgs      []string
	)

	BeforeEach(func() {
		binPath = "/image-plugin"
		extraArgs = []string{}
	})

	JustBeforeEach(func() {
		commandCreator = &imageplugin.DefaultCommandCreator{
			BinPath:   binPath,
			ExtraArgs: extraArgs,
		}
	})

	Describe("CreateCommand", func() {
		var (
			createCmd *exec.Cmd
			spec      rootfs_spec.Spec
		)

		BeforeEach(func() {
			rootfsURL, err := url.Parse("/fake-registry/image")
			Expect(err).NotTo(HaveOccurred())
			spec = rootfs_spec.Spec{RootFS: rootfsURL}
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

		It("returns a command with the provided rootfs as image", func() {
			Expect(createCmd.Args[2]).To(Equal("/fake-registry/image"))
		})

		It("returns a command with the provided handle as id", func() {
			Expect(createCmd.Args[3]).To(Equal("test-handle"))
		})

		Context("when using a docker image", func() {
			BeforeEach(func() {
				var err error
				spec.RootFS, err = url.Parse("docker:///busybox#1.26.1")
				Expect(err).NotTo(HaveOccurred())
			})

			It("replaces the '#' with ':'", func() {
				Expect(createCmd.Args[2]).To(Equal("docker:///busybox:1.26.1"))
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
					Expect(createCmd.Args[2]).To(Equal("--disk-limit-size-bytes"))
					Expect(createCmd.Args[3]).To(Equal("100000"))
				})

				Context("and it's got an exclusive scope", func() {
					BeforeEach(func() {
						spec.QuotaScope = garden.DiskLimitScopeExclusive
					})

					It("returns a command with the quota and an exclusive scope", func() {
						Expect(createCmd.Args[2]).To(Equal("--disk-limit-size-bytes"))
						Expect(createCmd.Args[3]).To(Equal("100000"))

						Expect(createCmd.Args).To(ContainElement("--exclude-image-from-quota"))
					})
				})

				Context("and it's got a total scope", func() {
					BeforeEach(func() {
						spec.QuotaScope = garden.DiskLimitScopeTotal
					})

					It("returns a command with the quota and a total scope", func() {
						Expect(createCmd.Args[2]).To(Equal("--disk-limit-size-bytes"))
						Expect(createCmd.Args[3]).To(Equal("100000"))

						Expect(createCmd.Args).NotTo(ContainElement("--exclude-image-from-quota"))
					})
				})
			})
		})

		Context("and username is not provided", func() {
			It("returns a command without the username", func() {
				Expect(createCmd.Args).NotTo(ContainElement("--username"))
			})
		})

		Context("when password is not provided", func() {
			It("returns a command without the password", func() {
				Expect(createCmd.Args).NotTo(ContainElement("--password"))
			})
		})

		Context("when username is provided", func() {
			BeforeEach(func() {
				spec.Username = "rootfsuser"
			})

			It("returns a command with the username", func() {
				Expect(createCmd.Args).To(ContainElement("--username"))
				Expect(createCmd.Args).To(ContainElement("rootfsuser"))
			})
		})

		Context("when password is provided", func() {
			BeforeEach(func() {
				spec.Password = "secretpassword"
			})

			It("returns a command with the password", func() {
				Expect(createCmd.Args).To(ContainElement("--password"))
				Expect(createCmd.Args).To(ContainElement("secretpassword"))
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

		It("returns a command that runs as the current user (SysProcAttr.Credential not set)", func() {
			Expect(createCmd.SysProcAttr).To(BeNil())
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

		It("returns a command that runs as the current user (SysProcAttr.Credential not set)", func() {
			Expect(destroyCmd.SysProcAttr).To(BeNil())
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

		It("returns a command that runs as the current user (SysProcAttr.Credential not set)", func() {
			Expect(metricsCmd.SysProcAttr).To(BeNil())
		})
	})
})

package imageplugin_test

import (
	"errors"
	"net/url"
	"os/exec"

	"code.cloudfoundry.org/garden-shed/rootfs_provider"
	"code.cloudfoundry.org/guardian/imageplugin"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("ExternalImageManager", func() {
	var (
		fakeCommandRunner    *fake_command_runner.FakeCommandRunner
		logger               *lagertest.TestLogger
		externalImageManager *imageplugin.ExternalImageManager
		imageSource          *url.URL
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("external-image-manager")
		fakeCommandRunner = fake_command_runner.New()
		externalImageManager = imageplugin.New("/external-image-manager-bin", "/var/vcap/data/external-image-manager", fakeCommandRunner)

		var err error
		imageSource, err = url.Parse("/hello/image")
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Create", func() {
		It("uses the correct external-image-manager binary", func() {
			_, _, err := externalImageManager.Create(logger, "hello", rootfs_provider.Spec{
				RootFS: imageSource,
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
			imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

			Expect(imageManagerCmd.Path).To(Equal("/external-image-manager-bin"))
		})

		It("uses the external-image-manager binary output as the rootpath return", func() {
			fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "/external-image-manager-bin",
			}, func(cmd *exec.Cmd) error {
				cmd.Stdout.Write([]byte("/this-is/your-rootfs"))
				cmd.Stderr.Write([]byte("/this-is-not/your-rootfs"))
				return nil
			})

			rootfs, _, err := externalImageManager.Create(logger, "hello", rootfs_provider.Spec{
				RootFS: imageSource,
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(rootfs).To(Equal("/this-is/your-rootfs"))
		})

		It("doesn't support image env yet", func() {
			_, envs, err := externalImageManager.Create(logger, "hello", rootfs_provider.Spec{
				RootFS: imageSource,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(envs).To(Equal([]string{}))
		})

		Context("external-image-manager parameters", func() {
			It("sets the correct external-image-manager store path", func() {
				_, _, err := externalImageManager.Create(logger, "hello", rootfs_provider.Spec{
					RootFS: imageSource,
				})
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(imageManagerCmd.Args[1]).To(Equal("--store"))
				Expect(imageManagerCmd.Args[2]).To(Equal("/var/vcap/data/external-image-manager"))
			})

			It("uses the correct external-image-manager create command", func() {
				_, _, err := externalImageManager.Create(logger, "hello", rootfs_provider.Spec{
					RootFS: imageSource,
				})
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(imageManagerCmd.Args[3]).To(Equal("create"))
			})

			It("sets the correct image input to external-image-manager", func() {
				_, _, err := externalImageManager.Create(logger, "hello", rootfs_provider.Spec{
					RootFS: imageSource,
				})
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(imageManagerCmd.Args[4]).To(Equal("/hello/image"))
			})

			It("sets the correct id to external-image-manager", func() {
				_, _, err := externalImageManager.Create(logger, "hello", rootfs_provider.Spec{
					RootFS: imageSource,
				})
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(imageManagerCmd.Args[5]).To(Equal("hello"))
			})
		})

		Context("when the command fails", func() {
			BeforeEach(func() {
				fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "/external-image-manager-bin",
				}, func(cmd *exec.Cmd) error {
					cmd.Stderr.Write([]byte("btrfs doesn't like you"))

					return errors.New("external-image-manager failure")
				})
			})

			It("returns an error", func() {
				_, _, err := externalImageManager.Create(logger, "hello", rootfs_provider.Spec{
					RootFS: imageSource,
				})
				Expect(err).To(MatchError(ContainSubstring("external image manager create failed")))
				Expect(err).To(MatchError(ContainSubstring("external-image-manager failure")))
			})

			It("returns the external-image-manager error output in the error", func() {
				externalImageManager.Create(logger, "hello", rootfs_provider.Spec{
					RootFS: imageSource,
				})
				Expect(logger).To(gbytes.Say("btrfs doesn't like you"))
			})
		})
	})

	Describe("Destroy", func() {
		It("uses the correct external-image-manager binary", func() {
			err := externalImageManager.Destroy(logger, "hello")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
			imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

			Expect(imageManagerCmd.Path).To(Equal("/external-image-manager-bin"))
		})

		Context("external-image-manager parameters", func() {
			It("sets the correct external-image-manager store path", func() {
				err := externalImageManager.Destroy(logger, "hello")
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(imageManagerCmd.Args[1]).To(Equal("--store"))
				Expect(imageManagerCmd.Args[2]).To(Equal("/var/vcap/data/external-image-manager"))
			})

			It("uses the correct external-image-manager delete command", func() {
				err := externalImageManager.Destroy(logger, "hello")
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(imageManagerCmd.Args[3]).To(Equal("delete"))
			})

			It("sets the correct id to external-image-manager", func() {
				err := externalImageManager.Destroy(logger, "hello")
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(imageManagerCmd.Args[4]).To(Equal("hello"))
			})
		})

		Context("when the command fails", func() {
			BeforeEach(func() {
				fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "/external-image-manager-bin",
				}, func(cmd *exec.Cmd) error {
					cmd.Stderr.Write([]byte("btrfs doesn't like you"))

					return errors.New("external-image-manager failure")
				})
			})

			It("returns an error", func() {
				err := externalImageManager.Destroy(logger, "hello")
				Expect(err).To(MatchError(ContainSubstring("external image manager destroy failed")))
				Expect(err).To(MatchError(ContainSubstring("external-image-manager failure")))
			})

			It("returns the external-image-manager error output in the error", func() {
				externalImageManager.Destroy(logger, "hello")
				Expect(logger).To(gbytes.Say("btrfs doesn't like you"))
			})
		})
	})
})

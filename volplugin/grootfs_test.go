package volplugin_test

import (
	"errors"
	"net/url"
	"os/exec"

	"code.cloudfoundry.org/garden-shed/rootfs_provider"
	"code.cloudfoundry.org/guardian/volplugin"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Grootfs", func() {
	var (
		fakeCommandRunner *fake_command_runner.FakeCommandRunner
		logger            *lagertest.TestLogger
		grootfsVC         *volplugin.GrootfsVC
		imageSource       *url.URL
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("grootfs")
		fakeCommandRunner = fake_command_runner.New()
		grootfsVC = volplugin.NewGrootfsVC("/grootfs-bin", "/var/vcap/data/grootfs", fakeCommandRunner)

		var err error
		imageSource, err = url.Parse("/hello/image")
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Create", func() {
		It("uses the correct grootfs binary", func() {
			_, _, err := grootfsVC.Create(logger, "hello", rootfs_provider.Spec{
				RootFS: imageSource,
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
			grootfsCmd := fakeCommandRunner.ExecutedCommands()[0]

			Expect(grootfsCmd.Path).To(Equal("/grootfs-bin"))
		})

		It("uses the grootfs binary output as the rootpath return", func() {
			fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "/grootfs-bin",
			}, func(cmd *exec.Cmd) error {
				cmd.Stdout.Write([]byte("/this-is/your-rootfs"))
				cmd.Stderr.Write([]byte("/this-is-not/your-rootfs"))
				return nil
			})

			rootfs, _, err := grootfsVC.Create(logger, "hello", rootfs_provider.Spec{
				RootFS: imageSource,
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(rootfs).To(Equal("/this-is/your-rootfs"))
		})

		It("doesn't support image env yet", func() {
			_, envs, err := grootfsVC.Create(logger, "hello", rootfs_provider.Spec{
				RootFS: imageSource,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(envs).To(Equal([]string{}))
		})

		Context("grootfs parameters", func() {
			It("sets the correct grootfs store path", func() {
				_, _, err := grootfsVC.Create(logger, "hello", rootfs_provider.Spec{
					RootFS: imageSource,
				})
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				grootfsCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(grootfsCmd.Args[1]).To(Equal("--store"))
				Expect(grootfsCmd.Args[2]).To(Equal("/var/vcap/data/grootfs"))
			})

			It("uses the correct grootfs create command", func() {
				_, _, err := grootfsVC.Create(logger, "hello", rootfs_provider.Spec{
					RootFS: imageSource,
				})
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				grootfsCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(grootfsCmd.Args[3]).To(Equal("create"))
			})

			It("sets the correct image input to grootfs", func() {
				_, _, err := grootfsVC.Create(logger, "hello", rootfs_provider.Spec{
					RootFS: imageSource,
				})
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				grootfsCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(grootfsCmd.Args[4]).To(Equal("/hello/image"))
			})

			It("sets the correct id to grootfs", func() {
				_, _, err := grootfsVC.Create(logger, "hello", rootfs_provider.Spec{
					RootFS: imageSource,
				})
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				grootfsCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(grootfsCmd.Args[5]).To(Equal("hello"))
			})
		})

		Context("when the command fails", func() {
			BeforeEach(func() {
				fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "/grootfs-bin",
				}, func(cmd *exec.Cmd) error {
					cmd.Stderr.Write([]byte("btrfs doesn't like you"))

					return errors.New("grootfs failure")
				})
			})

			It("returns an error", func() {
				_, _, err := grootfsVC.Create(logger, "hello", rootfs_provider.Spec{
					RootFS: imageSource,
				})
				Expect(err).To(MatchError(ContainSubstring("grootfs failure")))
			})

			It("returns the grootfs error output in the error", func() {
				_, _, err := grootfsVC.Create(logger, "hello", rootfs_provider.Spec{
					RootFS: imageSource,
				})
				Expect(err).To(MatchError(ContainSubstring("btrfs doesn't like you")))
			})
		})
	})

	Describe("Destroy", func() {
		It("uses the correct grootfs binary", func() {
			err := grootfsVC.Destroy(logger, "hello")
			Expect(err).ToNot(HaveOccurred())

			Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
			grootfsCmd := fakeCommandRunner.ExecutedCommands()[0]

			Expect(grootfsCmd.Path).To(Equal("/grootfs-bin"))
		})

		Context("grootfs parameters", func() {
			It("sets the correct grootfs store path", func() {
				err := grootfsVC.Destroy(logger, "hello")
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				grootfsCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(grootfsCmd.Args[1]).To(Equal("--store"))
				Expect(grootfsCmd.Args[2]).To(Equal("/var/vcap/data/grootfs"))
			})

			It("uses the correct grootfs delete command", func() {
				err := grootfsVC.Destroy(logger, "hello")
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				grootfsCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(grootfsCmd.Args[3]).To(Equal("delete"))
			})

			It("sets the correct id to grootfs", func() {
				err := grootfsVC.Destroy(logger, "hello")
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				grootfsCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(grootfsCmd.Args[4]).To(Equal("hello"))
			})
		})

		Context("when the command fails", func() {
			BeforeEach(func() {
				fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "/grootfs-bin",
				}, func(cmd *exec.Cmd) error {
					cmd.Stderr.Write([]byte("btrfs doesn't like you"))

					return errors.New("grootfs failure")
				})
			})

			It("returns an error", func() {
				err := grootfsVC.Destroy(logger, "hello")
				Expect(err).To(MatchError(ContainSubstring("grootfs failure")))
			})

			It("returns the grootfs error output in the error", func() {
				err := grootfsVC.Destroy(logger, "hello")
				Expect(err).To(MatchError(ContainSubstring("btrfs doesn't like you")))
			})
		})
	})
})

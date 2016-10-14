package imageplugin_test

import (
	"errors"
	"net/url"
	"os/exec"
	"strconv"

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
		externalImageManager = imageplugin.New("/external-image-manager-bin", fakeCommandRunner)

		var err error
		imageSource, err = url.Parse("/hello/image")
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Create", func() {
		var (
			returnedRootFS string
			testQuotaSize  int64
			err            error
		)

		BeforeEach(func() {
			testQuotaSize = 0
		})

		JustBeforeEach(func() {
			returnedRootFS, _, err = externalImageManager.Create(logger, "hello", rootfs_provider.Spec{
				QuotaSize: testQuotaSize,
				RootFS:    imageSource,
			})
		})

		It("uses the correct external-image-manager binary", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
			imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

			Expect(imageManagerCmd.Path).To(Equal("/external-image-manager-bin"))
		})

		Describe("external-image-manager parameters", func() {
			It("uses the correct external-image-manager create command", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(imageManagerCmd.Args[1]).To(Equal("create"))
			})

			It("sets the correct image input to external-image-manager", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(imageManagerCmd.Args[len(imageManagerCmd.Args)-2]).To(Equal("/hello/image"))
			})

			It("sets the correct id to external-image-manager", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(imageManagerCmd.Args[len(imageManagerCmd.Args)-1]).To(Equal("hello"))
			})

			Context("when a disk quota is provided in the spec", func() {
				BeforeEach(func() {
					testQuotaSize = 1024
				})

				It("passes the quota to the external-image-manager", func() {
					Expect(err).ToNot(HaveOccurred())
					Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
					imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

					Expect(imageManagerCmd.Args[2]).To(Equal("--disk-limit-size-bytes"))
					Expect(imageManagerCmd.Args[3]).To(Equal(strconv.FormatInt(testQuotaSize, 10)))
				})
			})
		})

		Context("when the external-image-manager binary prints to stdout/stderr", func() {
			BeforeEach(func() {
				fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "/external-image-manager-bin",
				}, func(cmd *exec.Cmd) error {
					cmd.Stdout.Write([]byte("/this-is/your-rootfs"))
					cmd.Stderr.Write([]byte("/this-is-not/your-rootfs"))
					return nil
				})
			})

			It("returns stdout as the rootfs location", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(returnedRootFS).To(Equal("/this-is/your-rootfs"))
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
				Expect(err).To(MatchError(ContainSubstring("external image manager create failed")))
				Expect(err).To(MatchError(ContainSubstring("external-image-manager failure")))
			})

			It("returns the external-image-manager error output in the error", func() {
				Expect(logger).To(gbytes.Say("btrfs doesn't like you"))
			})
		})
	})

	Describe("Destroy", func() {
		var err error

		JustBeforeEach(func() {
			err = externalImageManager.Destroy(logger, "hello")
		})

		It("uses the correct external-image-manager binary", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
			imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

			Expect(imageManagerCmd.Path).To(Equal("/external-image-manager-bin"))
		})

		Describe("external-image-manager parameters", func() {
			It("uses the correct external-image-manager delete command", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(imageManagerCmd.Args[1]).To(Equal("delete"))
			})

			It("sets the correct id to external-image-manager", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(imageManagerCmd.Args[len(imageManagerCmd.Args)-1]).To(Equal("hello"))
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
				Expect(err).To(MatchError(ContainSubstring("external image manager destroy failed")))
				Expect(err).To(MatchError(ContainSubstring("external-image-manager failure")))
			})

			It("returns the external-image-manager error output in the error", func() {
				Expect(logger).To(gbytes.Say("btrfs doesn't like you"))
			})
		})
	})
})

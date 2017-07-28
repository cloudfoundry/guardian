package limiter_test

import (
	"errors"
	"os/exec"

	limiterpkg "code.cloudfoundry.org/grootfs/store/filesystems/btrfs/drax/limiter"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	. "code.cloudfoundry.org/commandrunner/fake_command_runner/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Btrfs", func() {
	var (
		fakeCommandRunner *fake_command_runner.FakeCommandRunner
		limiter           *limiterpkg.BtrfsLimiter
		logger            lager.Logger
	)

	BeforeEach(func() {
		fakeCommandRunner = fake_command_runner.New()

		limiter = limiterpkg.NewBtrfsLimiter("custom-btrfs-bin", fakeCommandRunner)

		logger = lagertest.NewTestLogger("drax-limiter")
	})

	Describe("ApplyDiskLimit", func() {

		It("limits the provided volume", func() {
			Expect(limiter.ApplyDiskLimit(logger, "/full/path/to/volume", 1024*1024, false)).To(Succeed())

			Expect(fakeCommandRunner).Should(HaveExecutedSerially(fake_command_runner.CommandSpec{
				Path: "custom-btrfs-bin",
				Args: []string{"qgroup", "limit", "1048576", "/full/path/to/volume"},
			}))
		})

		Context("when the exclusive limit flag is provided", func() {
			It("applies the quota exclusively", func() {
				Expect(limiter.ApplyDiskLimit(logger, "/full/path/to/volume", 1024*1024, true)).To(Succeed())

				Expect(fakeCommandRunner).Should(HaveExecutedSerially(fake_command_runner.CommandSpec{
					Path: "custom-btrfs-bin",
					Args: []string{"qgroup", "limit", "-e", "1048576", "/full/path/to/volume"},
				}))
			})
		})

		Context("when setting the limit fails", func() {
			BeforeEach(func() {
				fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "custom-btrfs-bin",
				}, func(cmd *exec.Cmd) error {
					_, err := cmd.Stdout.Write([]byte("failed to set btrfs limit"))
					Expect(err).NotTo(HaveOccurred())
					_, err = cmd.Stderr.Write([]byte("some stderr text"))
					Expect(err).NotTo(HaveOccurred())

					return errors.New("exit status 1")
				})
			})

			It("forwards the stdout and stderr", func() {
				err := limiter.ApplyDiskLimit(logger, "/full/path/to/volume", 1024*1024, false)

				Expect(err).To(MatchError(ContainSubstring("failed to set btrfs limit")))
				Expect(err).To(MatchError(ContainSubstring("some stderr text")))
			})
		})
	})

	Describe("DestroyQuotaGroup", func() {
		It("destroys the qgroup for the path", func() {
			Expect(limiter.DestroyQuotaGroup(logger, "/full/path/to/volume")).To(Succeed())

			Expect(fakeCommandRunner).Should(HaveExecutedSerially(fake_command_runner.CommandSpec{
				Path: "custom-btrfs-bin",
				Args: []string{"qgroup", "destroy", "/full/path/to/volume", "/full/path/to/volume"},
			}))
		})

		Context("when destroying the qgroup fails", func() {
			BeforeEach(func() {
				fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "custom-btrfs-bin",
				}, func(cmd *exec.Cmd) error {
					_, err := cmd.Stdout.Write([]byte("failed to destroy qgroup"))
					Expect(err).NotTo(HaveOccurred())
					_, err = cmd.Stderr.Write([]byte("some stderr text"))
					Expect(err).NotTo(HaveOccurred())

					return errors.New("exit status 1")
				})
			})

			It("forwards the stdout and stderr", func() {
				err := limiter.DestroyQuotaGroup(logger, "/full/path/to/volume")

				Expect(err).To(MatchError(ContainSubstring("failed to destroy qgroup")))
				Expect(err).To(MatchError(ContainSubstring("some stderr text")))
			})
		})
	})
})

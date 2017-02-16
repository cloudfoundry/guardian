package unpacker_test

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	"code.cloudfoundry.org/grootfs/base_image_puller"
	unpackerpkg "code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/st3v/glager"
)

var _ = Describe("NSSysProcUnpacker", func() {
	var (
		fakeCommandRunner *fake_command_runner.FakeCommandRunner
		unpacker          *unpackerpkg.NSSysProcUnpacker

		logger     *TestLogger
		imagePath  string
		targetPath string
		filesystem string

		commandError             error
		whenCommandRunnerRunning func(cmd *exec.Cmd) error
	)

	BeforeEach(func() {
		var err error
		whenCommandRunnerRunning = func(cmd *exec.Cmd) error {
			return nil
		}

		filesystem = "btrfs"
		fakeCommandRunner = fake_command_runner.New()
		unpacker = unpackerpkg.NewNSSysProcUnpacker(fakeCommandRunner, filesystem)

		logger = NewLogger("test-store")

		imagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		targetPath = filepath.Join(imagePath, "rootfs")

		commandError = nil
	})

	JustBeforeEach(func() {
		fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
			Path: "/proc/self/exe",
		}, func(cmd *exec.Cmd) error {
			cmd.Process = &os.Process{
				Pid: 12, // don't panic
			}

			if err := whenCommandRunnerRunning(cmd); err != nil {
				return err
			}

			return commandError
		})
	})

	AfterEach(func() {
		Expect(os.RemoveAll(imagePath)).To(Succeed())
	})

	It("passes the rootfs path and filesystem to the unpack command", func() {
		Expect(unpacker.Unpack(logger, base_image_puller.UnpackSpec{
			TargetPath: targetPath,
		})).To(Succeed())

		commands := fakeCommandRunner.ExecutedCommands()
		Expect(commands).To(HaveLen(1))
		Expect(commands[0].Path).To(Equal("/proc/self/exe"))
		Expect(commands[0].Args).To(Equal([]string{
			"unpack", targetPath, filesystem,
		}))
	})

	It("passes the provided stream to the unpack command", func() {
		streamR, streamW, err := os.Pipe()
		Expect(err).NotTo(HaveOccurred())

		Expect(unpacker.Unpack(logger, base_image_puller.UnpackSpec{
			Stream:     streamR,
			TargetPath: targetPath,
		})).To(Succeed())

		_, err = streamW.WriteString("hello-world")
		Expect(err).NotTo(HaveOccurred())
		Expect(streamW.Close()).To(Succeed())

		commands := fakeCommandRunner.ExecutedCommands()
		Expect(commands).To(HaveLen(1))

		contents, err := ioutil.ReadAll(commands[0].Stdin)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("hello-world"))
	})

	It("starts the unpack command in a user namespace with id mappings", func() {
		Expect(unpacker.Unpack(logger, base_image_puller.UnpackSpec{
			UIDMappings: []groot.IDMappingSpec{
				{HostID: 1000, NamespaceID: 2000, Size: 10},
			},
			TargetPath: targetPath,
		})).To(Succeed())

		commands := fakeCommandRunner.ExecutedCommands()
		Expect(commands).To(HaveLen(1))
		Expect(commands[0].SysProcAttr.Cloneflags).To(Equal(uintptr(syscall.CLONE_NEWUSER)))
		Expect(commands[0].SysProcAttr.UidMappings).To(Equal([]syscall.SysProcIDMap{
			{HostID: 1000, ContainerID: 2000, Size: 10},
		}))
	})

	It("re-logs the log lines emitted by the unpack-wrapper command", func() {
		whenCommandRunnerRunning = func(cmd *exec.Cmd) error {
			logger := lager.NewLogger("fake-unpack")
			logger.RegisterSink(lager.NewWriterSink(cmd.Stderr, lager.DEBUG))
			logger.Debug("foo")
			logger.Info("bar")

			return nil
		}

		Expect(unpacker.Unpack(logger, base_image_puller.UnpackSpec{
			TargetPath: targetPath,
		})).To(Succeed())

		Expect(logger).To(ContainSequence(
			Debug(
				Message("test-store.ns-sysproc-unpacking.fake-unpack.foo"),
			),
			Info(
				Message("test-store.ns-sysproc-unpacking.fake-unpack.bar"),
			),
		))
	})

	Context("when no mappings are provided", func() {
		It("starts the unpack command in the same namespaces and with no mappings", func() {
			Expect(unpacker.Unpack(logger, base_image_puller.UnpackSpec{
				TargetPath: targetPath,
			})).To(Succeed())

			commands := fakeCommandRunner.ExecutedCommands()
			Expect(commands).To(HaveLen(1))
			Expect(commands[0].SysProcAttr.Cloneflags).To(Equal(uintptr(0)))
			Expect(commands[0].SysProcAttr.UidMappings).To(HaveLen(0))
		})
	})

	Context("when unpack command fails", func() {
		BeforeEach(func() {
			commandError = errors.New("failed to start unpack")
		})

		It("returns an error", func() {
			Expect(unpacker.Unpack(logger, base_image_puller.UnpackSpec{
				TargetPath: targetPath,
			})).To(
				MatchError(ContainSubstring("failed to start unpack")),
			)
		})
	})

	Context("when the unpack command fails", func() {
		BeforeEach(func() {
			whenCommandRunnerRunning = func(cmd *exec.Cmd) error {
				cmd.Stdout.Write([]byte("hello-world"))
				return errors.New("exit status 1")
			}
		})

		It("returns an error", func() {
			Expect(unpacker.Unpack(logger, base_image_puller.UnpackSpec{
				TargetPath: targetPath,
			})).NotTo(Succeed())
		})

		It("returns the command output", func() {
			Expect(unpacker.Unpack(logger, base_image_puller.UnpackSpec{
				TargetPath: targetPath,
			})).To(
				MatchError(ContainSubstring("hello-world")),
			)
		})
	})
})

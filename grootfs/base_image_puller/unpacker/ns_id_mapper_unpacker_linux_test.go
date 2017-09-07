package unpacker_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	"code.cloudfoundry.org/grootfs/base_image_puller"
	unpackerpkg "code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/grootfs/base_image_puller/unpacker/unpackerfakes"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/st3v/glager"
)

var _ = Describe("NSIdMapperUnpacker", func() {
	var (
		fakeIDMapper      *unpackerfakes.FakeIDMapper
		fakeCommandRunner *fake_command_runner.FakeCommandRunner
		unpacker          *unpackerpkg.NSIdMapperUnpacker

		logger         *TestLogger
		imagePath      string
		targetPath     string
		unpackStrategy unpackerpkg.UnpackStrategy

		commandError error
		reexecOutput string
	)

	BeforeEach(func() {
		var err error

		fakeIDMapper = new(unpackerfakes.FakeIDMapper)
		fakeCommandRunner = fake_command_runner.New()
		unpackStrategy.Name = "btrfs"
		unpacker = unpackerpkg.NewNSIdMapperUnpacker(fakeCommandRunner, fakeIDMapper, unpackStrategy)
		reexecOutput = ""

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

			if reexecOutput != "" {
				_, err := cmd.Stdout.Write([]byte(reexecOutput))
				Expect(err).NotTo(HaveOccurred())
			}

			Expect(json.NewEncoder(cmd.Stdout).Encode(base_image_puller.UnpackOutput{BytesWritten: 1024})).To(Succeed())
			return commandError
		})
	})

	AfterEach(func() {
		Expect(os.RemoveAll(imagePath)).To(Succeed())
	})

	It("passes the rootfs path, base-directory and filesystem to the unpack command", func() {
		_, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
			TargetPath:    targetPath,
			BaseDirectory: "/base-folder/",
		})
		Expect(err).NotTo(HaveOccurred())

		unpackStrategyJson, err := json.Marshal(&unpackStrategy)
		Expect(err).NotTo(HaveOccurred())

		commands := fakeCommandRunner.StartedCommands()
		Expect(commands).To(HaveLen(1))
		Expect(commands[0].Path).To(Equal("/proc/self/exe"))
		Expect(commands[0].Args).To(Equal([]string{
			"unpack", targetPath, "/base-folder/", string(unpackStrategyJson),
		}))
	})

	It("returns the total bytes written based on the unpack output", func() {
		totalBytes, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
			TargetPath: targetPath,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(totalBytes).To(Equal(base_image_puller.UnpackOutput{BytesWritten: 1024}))
	})

	It("passes the provided stream to the unpack command", func() {
		streamR, streamW, err := os.Pipe()
		Expect(err).NotTo(HaveOccurred())

		_, err = unpacker.Unpack(logger, base_image_puller.UnpackSpec{
			Stream:     streamR,
			TargetPath: targetPath,
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = streamW.WriteString("hello-world")
		Expect(err).NotTo(HaveOccurred())
		Expect(streamW.Close()).To(Succeed())

		commands := fakeCommandRunner.StartedCommands()
		Expect(commands).To(HaveLen(1))

		contents, err := ioutil.ReadAll(commands[0].Stdin)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("hello-world"))
	})

	It("starts the unpack command in a user namespace", func() {
		_, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
			UIDMappings: []groot.IDMappingSpec{
				{HostID: 1000, NamespaceID: 2000, Size: 10},
			},
			TargetPath: targetPath,
		})
		Expect(err).NotTo(HaveOccurred())

		commands := fakeCommandRunner.StartedCommands()
		Expect(commands).To(HaveLen(1))
		Expect(commands[0].SysProcAttr.Cloneflags).To(Equal(uintptr(syscall.CLONE_NEWUSER)))
	})

	It("re-logs the log lines emitted by the unpack command", func() {
		fakeCommandRunner.WhenWaitingFor(fake_command_runner.CommandSpec{
			Path: "/proc/self/exe",
		}, func(cmd *exec.Cmd) error {
			logger := lager.NewLogger("fake-unpack")
			logger.RegisterSink(lager.NewWriterSink(cmd.Stderr, lager.DEBUG))
			logger.Debug("foo")
			logger.Info("bar")
			return nil
		})

		_, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
			TargetPath: targetPath,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(logger).To(ContainSequence(
			Debug(
				Message("test-store.ns-id-mapper-unpacking.fake-unpack.foo"),
			),
			Info(
				Message("test-store.ns-id-mapper-unpacking.fake-unpack.bar"),
			),
		))
	})

	Context("when the unpack prints invalid output", func() {
		It("returns an error", func() {
			reexecOutput = "not a valid thing {{}))"
			_, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
				TargetPath: targetPath,
			})
			Expect(err).To(MatchError(ContainSubstring("invalid unpack output")))
		})
	})

	Context("when no mappings are provided", func() {
		It("starts the unpack command in the same namespaces", func() {
			_, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
				TargetPath: targetPath,
			})
			Expect(err).NotTo(HaveOccurred())

			commands := fakeCommandRunner.StartedCommands()
			Expect(commands).To(HaveLen(1))
			Expect(commands[0].SysProcAttr.Cloneflags).To(Equal(uintptr(0)))
		})
	})

	It("signals the unpack command to continue using the contol pipe", func(done Done) {
		_, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
			TargetPath: targetPath,
		})
		Expect(err).NotTo(HaveOccurred())

		commands := fakeCommandRunner.StartedCommands()
		Expect(commands).To(HaveLen(1))
		buffer := make([]byte, 1)
		_, err = commands[0].ExtraFiles[0].Read(buffer)
		Expect(err).NotTo(HaveOccurred())

		close(done)
	}, 1.0)

	Describe("UIDMappings", func() {
		It("applies the provided uid mappings", func() {
			_, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
				TargetPath: targetPath,
				UIDMappings: []groot.IDMappingSpec{
					{HostID: 1000, NamespaceID: 2000, Size: 10},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeIDMapper.MapUIDsCallCount()).To(Equal(1))
			_, _, mappings := fakeIDMapper.MapUIDsArgsForCall(0)

			Expect(mappings).To(Equal([]groot.IDMappingSpec{
				{HostID: 1000, NamespaceID: 2000, Size: 10},
			}))
		})

		Context("when applying the mappings fails", func() {
			BeforeEach(func() {
				fakeIDMapper.MapUIDsReturns(errors.New("Boom!"))
			})

			It("closes the control pipe", func() {
				_, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
					TargetPath: targetPath,
					UIDMappings: []groot.IDMappingSpec{
						{HostID: 1000, NamespaceID: 2000, Size: 10},
					},
				})
				Expect(err).To(HaveOccurred())

				commands := fakeCommandRunner.StartedCommands()
				Expect(commands).To(HaveLen(1))
				buffer := make([]byte, 1)
				_, err = commands[0].ExtraFiles[0].Read(buffer)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("GIDMappings", func() {
		It("applies the provided gid mappings", func() {
			_, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
				TargetPath: targetPath,
				GIDMappings: []groot.IDMappingSpec{
					{HostID: 1000, NamespaceID: 2000, Size: 10},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeIDMapper.MapGIDsCallCount()).To(Equal(1))
			_, _, mappings := fakeIDMapper.MapGIDsArgsForCall(0)

			Expect(mappings).To(Equal([]groot.IDMappingSpec{
				{HostID: 1000, NamespaceID: 2000, Size: 10},
			}))
		})

		Context("when applying the mappings fails", func() {
			BeforeEach(func() {
				fakeIDMapper.MapGIDsReturns(errors.New("Boom!"))
			})

			It("closes the control pipe", func() {
				_, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
					TargetPath: targetPath,
					GIDMappings: []groot.IDMappingSpec{
						{HostID: 1000, NamespaceID: 2000, Size: 10},
					},
				})
				Expect(err).To(HaveOccurred())

				commands := fakeCommandRunner.StartedCommands()
				Expect(commands).To(HaveLen(1))
				buffer := make([]byte, 1)
				_, err = commands[0].ExtraFiles[0].Read(buffer)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("when it fails to start the unpack command", func() {
		BeforeEach(func() {
			commandError = errors.New("failed to start unpack")
		})

		It("returns an error", func() {
			_, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
				TargetPath: targetPath,
			})
			Expect(err).To(MatchError(ContainSubstring("failed to start unpack")))
		})
	})

	Context("when the unpack command fails", func() {
		BeforeEach(func() {
			fakeCommandRunner.WhenWaitingFor(fake_command_runner.CommandSpec{
				Path: "/proc/self/exe",
			}, func(cmd *exec.Cmd) error {
				_, err := cmd.Stdout.Write([]byte("hello-world"))
				Expect(err).NotTo(HaveOccurred())
				return errors.New("exit status 1")
			})
		})

		It("returns an error", func() {
			_, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
				TargetPath: targetPath,
			})
			Expect(err).To(HaveOccurred())
		})

		It("returns the command output", func() {
			_, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
				TargetPath: targetPath,
			})
			Expect(err).To(MatchError(ContainSubstring("hello-world")))
		})
	})
})

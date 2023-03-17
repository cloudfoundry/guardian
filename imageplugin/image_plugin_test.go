package imageplugin_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/imageplugin"
	fakes "code.cloudfoundry.org/guardian/imageplugin/imagepluginfakes"
	"code.cloudfoundry.org/lager/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/st3v/glager"
)

var _ = Describe("ImagePlugin", func() {

	var (
		imagePlugin imageplugin.ImagePlugin

		fakeUnprivilegedCommandCreator *fakes.FakeCommandCreator
		fakePrivilegedCommandCreator   *fakes.FakeCommandCreator
		fakeImageSpecCreator           *fakes.FakeImageSpecCreator
		fakeCommandRunner              *fake_command_runner.FakeCommandRunner

		fakeLogger lager.Logger

		defaultRootfs string
	)

	BeforeEach(func() {
		fakeUnprivilegedCommandCreator = new(fakes.FakeCommandCreator)
		fakePrivilegedCommandCreator = new(fakes.FakeCommandCreator)
		fakeImageSpecCreator = new(fakes.FakeImageSpecCreator)
		fakeCommandRunner = fake_command_runner.New()

		fakeLogger = glager.NewLogger("image-plugin")

		defaultRootfs = "/default-rootfs"
	})

	JustBeforeEach(func() {
		imagePlugin = imageplugin.ImagePlugin{
			UnprivilegedCommandCreator: fakeUnprivilegedCommandCreator,
			PrivilegedCommandCreator:   fakePrivilegedCommandCreator,
			ImageSpecCreator:           fakeImageSpecCreator,
			CommandRunner:              fakeCommandRunner,
			DefaultRootfs:              defaultRootfs,
		}
	})

	Describe("Create", func() {
		var (
			cmd *exec.Cmd

			handle             string
			rootfsProviderSpec gardener.RootfsSpec
			rootfs             string
			namespaced         bool

			// fakeImagePluginStdout will override these if set
			createdSpec specs.Spec

			fakeImagePluginStdout string
			fakeImagePluginStderr string
			fakeImagePluginError  error

			baseRuntimeSpec specs.Spec
			createErr       error
		)

		BeforeEach(func() {
			cmd = exec.Command("unpriv-plugin", "create")
			fakeUnprivilegedCommandCreator.CreateCommandReturns(cmd, nil)
			fakePrivilegedCommandCreator.CreateCommandReturns(cmd, nil)

			handle = "test-handle"
			rootfs = "docker:///busybox"
			namespaced = true //assume unprivileged by default

			createdSpec = specs.Spec{
				Root: &specs.Root{Path: "/image-rootfs/rootfs"},
			}

			fakeImagePluginStdout = ""
			fakeImagePluginStderr = ""
			fakeImagePluginError = nil

			createErr = nil
		})

		JustBeforeEach(func() {
			fakeCommandRunner.WhenRunning(
				fake_command_runner.CommandSpec{
					Path: cmd.Path,
				},
				func(cmd *exec.Cmd) error {
					cmd.Stderr.Write([]byte(fakeImagePluginStderr))
					if fakeImagePluginStdout == "" {
						b, err := json.Marshal(createdSpec)
						Expect(err).NotTo(HaveOccurred())
						fakeImagePluginStdout = string(b)
					}

					cmd.Stdout.Write([]byte(fakeImagePluginStdout))
					return fakeImagePluginError
				},
			)

			rootfsURL, err := url.Parse(rootfs)
			Expect(err).NotTo(HaveOccurred())
			rootfsProviderSpec = gardener.RootfsSpec{RootFS: rootfsURL, Namespaced: namespaced}
			baseRuntimeSpec, createErr = imagePlugin.Create(fakeLogger, handle, rootfsProviderSpec)
		})

		It("calls the unprivileged command creator to generate a create command", func() {
			Expect(createErr).NotTo(HaveOccurred())
			Expect(fakeUnprivilegedCommandCreator.CreateCommandCallCount()).To(Equal(1))
			Expect(fakePrivilegedCommandCreator.CreateCommandCallCount()).To(Equal(0))

			_, handleArg, specArg := fakeUnprivilegedCommandCreator.CreateCommandArgsForCall(0)
			Expect(handleArg).To(Equal(handle))
			Expect(specArg).To(Equal(rootfsProviderSpec))
		})

		It("doesn't generate an OCI image spec", func() {
			Expect(fakeImageSpecCreator.CreateImageSpecCallCount()).To(Equal(0))
		})

		Context("when the unprivileged command creator returns an error", func() {
			BeforeEach(func() {
				fakeUnprivilegedCommandCreator.CreateCommandReturns(nil, errors.New("explosion"))
			})

			It("returns that error", func() {
				Expect(createErr).To(MatchError("creating create command: explosion"))
			})
		})

		Context("when the rootfs uses the scheme preloaded+layer", func() {
			var ociImageURI *url.URL

			BeforeEach(func() {
				var err error
				ociImageURI, err = url.Parse("https://arbitrary.com/this-must-be-a-parseable-url-though")
				Expect(err).NotTo(HaveOccurred())
				fakeImageSpecCreator.CreateImageSpecReturns(ociImageURI, nil)
				rootfs = "preloaded+layer:///path/to/rootfs?layer=https://layer.com/layer.tgz&layer_path=/a/path"
			})

			It("generates an OCI image spec", func() {
				Expect(fakeImageSpecCreator.CreateImageSpecCallCount()).To(Equal(1))
				actualRootFS, actualHandle := fakeImageSpecCreator.CreateImageSpecArgsForCall(0)
				Expect(actualRootFS.String()).To(Equal(rootfs))
				Expect(actualHandle).To(Equal(handle))
			})

			It("passes the new OCI image URI to the image plugin command creator", func() {
				Expect(fakeUnprivilegedCommandCreator.CreateCommandCallCount()).To(Equal(1))
				_, _, actualSpec := fakeUnprivilegedCommandCreator.CreateCommandArgsForCall(0)
				Expect(actualSpec.RootFS).To(Equal(ociImageURI))
			})

			Context("when the image spec creator returns an error", func() {
				BeforeEach(func() {
					fakeImageSpecCreator.CreateImageSpecReturns(nil, errors.New("morty"))
				})

				It("returns an error", func() {
					Expect(createErr).To(MatchError(ContainSubstring("morty")))
				})
			})
		})

		Context("when creating an unprivileged volume", func() {
			BeforeEach(func() {
				namespaced = false
			})

			It("calls the privileged command creator to generate a create command", func() {
				Expect(createErr).NotTo(HaveOccurred())
				Expect(fakePrivilegedCommandCreator.CreateCommandCallCount()).To(Equal(1))
				Expect(fakeUnprivilegedCommandCreator.CreateCommandCallCount()).To(Equal(0))

				_, handleArg, specArg := fakePrivilegedCommandCreator.CreateCommandArgsForCall(0)
				Expect(handleArg).To(Equal(handle))
				Expect(specArg).To(Equal(rootfsProviderSpec))
			})

			Context("when the privileged command creator returns an error", func() {
				BeforeEach(func() {
					fakePrivilegedCommandCreator.CreateCommandReturns(nil, errors.New("explosion"))
				})

				It("returns that error", func() {
					Expect(createErr).To(MatchError("creating create command: explosion"))
				})
			})
		})

		Context("when spec.Rootfs is not defined", func() {
			BeforeEach(func() {
				rootfs = ""
			})

			It("uses the default rootfs instead", func() {
				Expect(createErr).NotTo(HaveOccurred())
				Expect(fakeUnprivilegedCommandCreator.CreateCommandCallCount()).To(Equal(1))

				_, _, specArg := fakeUnprivilegedCommandCreator.CreateCommandArgsForCall(0)
				Expect(specArg.RootFS.String()).To(Equal("/default-rootfs"))
			})

			Context("when there is an error parsing the default rootfs", func() {
				BeforeEach(func() {
					defaultRootfs = "%%"
				})

				It("returns the error", func() {
					Expect(createErr).To(MatchError(ContainSubstring("parsing default rootfs")))
				})
			})
		})

		It("runs the plugin command with the command runner", func() {
			Expect(createErr).NotTo(HaveOccurred())
			Expect(fakeCommandRunner.ExecutedCommands()).To(HaveLen(1))
			executedCmd := fakeCommandRunner.ExecutedCommands()[0]

			Expect(executedCmd).To(Equal(cmd))
		})

		Context("when running the image plugin create fails", func() {
			BeforeEach(func() {
				fakeImagePluginStdout = "image-plugin-exploded-due-to-oom"
				fakeImagePluginError = errors.New("image-plugin-create-failed")
			})

			It("returns the wrapped error and plugin stdout, with context", func() {
				Expect(createErr).To(MatchError("running image plugin create: image-plugin-exploded-due-to-oom: image-plugin-create-failed"))
			})
		})

		It("returns the rootfs json property as the rootfs", func() {
			Expect(baseRuntimeSpec.Root.Path).To(Equal("/image-rootfs/rootfs"))
		})

		Context("when parsing the plugin output fails", func() {
			BeforeEach(func() {
				fakeImagePluginStdout = "THIS-IS-GARBAGE-OUTPUT"
			})

			It("returns the wrapped error and plugin stdout, with context", func() {
				Expect(createErr.Error()).To(ContainSubstring("parsing image plugin create: THIS-IS-GARBAGE-OUTPUT"))
			})
		})

		Context("when no image config is defined", func() {
			It("returns an empty list of env vars", func() {
				Expect(baseRuntimeSpec.Process.Env).To(BeEmpty())
			})
		})

		Context("when no rootfs path is returned", func() {
			BeforeEach(func() {
				createdSpec.Root = nil
			})

			It("returns an empty string for Root.Path", func() {
				Expect(baseRuntimeSpec.Root.Path).To(BeEmpty())
			})
		})

		Context("when there is env defined", func() {
			BeforeEach(func() {
				createdSpec.Process = &specs.Process{
					Env: []string{
						"MY_VAR=set",
						"MY_SECOND_VAR=also_set",
					},
				}
			})

			It("returns the list of env variables to set", func() {
				Expect(baseRuntimeSpec.Process.Env).To(ConsistOf([]string{"MY_VAR=set", "MY_SECOND_VAR=also_set"}))
			})
		})

		Context("when there are mounts defined", func() {
			BeforeEach(func() {
				createdSpec.Mounts = []specs.Mount{
					{
						Source:      "src",
						Destination: "dest",
						Options:     []string{"bind"},
						Type:        "bind",
					},
				}
			})

			It("returns the list of mounts to configure", func() {
				Expect(baseRuntimeSpec.Mounts).To(Equal(createdSpec.Mounts))
			})
		})

		Context("when the image plugin emits logs to stderr", func() {
			BeforeEach(func() {
				buffer := gbytes.NewBuffer()
				externalLogger := lager.NewLogger("external-plugin")
				externalLogger.RegisterSink(lager.NewWriterSink(buffer, lager.DEBUG))
				externalLogger.Info("info-message", lager.Data{"type": "info"})

				fakeImagePluginStderr = string(buffer.Contents())
			})

			It("relogs the log entries", func() {
				Expect(fakeLogger).To(glager.ContainSequence(
					glager.Info(
						glager.Message("image-plugin.image-plugin-create.external-plugin.info-message"),
						glager.Data("type", "info"),
					),
				))
			})
		})
	})

	Describe("Destroy", func() {
		var (
			unprivCmd *exec.Cmd
			privCmd   *exec.Cmd

			handle string

			fakeUnprivImagePluginStdout string
			fakeUnprivImagePluginStderr string
			fakeUnprivImagePluginError  error

			fakePrivImagePluginStdout string
			fakePrivImagePluginStderr string
			fakePrivImagePluginError  error

			destroyErr error
		)

		BeforeEach(func() {
			unprivCmd = exec.Command("unpriv-plugin", "destroy")
			privCmd = exec.Command("priv-plugin", "destroy")
			fakeUnprivilegedCommandCreator.DestroyCommandReturns(unprivCmd)
			fakePrivilegedCommandCreator.DestroyCommandReturns(privCmd)

			handle = "test-handle"

			fakeUnprivImagePluginStdout = ""
			fakeUnprivImagePluginStderr = ""
			fakeUnprivImagePluginError = nil

			fakePrivImagePluginStdout = ""
			fakePrivImagePluginStderr = ""
			fakePrivImagePluginError = nil

			destroyErr = nil
		})

		JustBeforeEach(func() {
			fakeCommandRunner.WhenRunning(
				fake_command_runner.CommandSpec{
					Path: unprivCmd.Path,
				},
				func(cmd *exec.Cmd) error {
					cmd.Stdout.Write([]byte(fakeUnprivImagePluginStdout))
					cmd.Stderr.Write([]byte(fakeUnprivImagePluginStderr))
					return fakeUnprivImagePluginError
				},
			)

			fakeCommandRunner.WhenRunning(
				fake_command_runner.CommandSpec{
					Path: privCmd.Path,
				},
				func(cmd *exec.Cmd) error {
					cmd.Stdout.Write([]byte(fakePrivImagePluginStdout))
					cmd.Stderr.Write([]byte(fakePrivImagePluginStderr))
					return fakePrivImagePluginError
				},
			)

			destroyErr = imagePlugin.Destroy(fakeLogger, handle)
		})

		Describe("Running the unprivileged plugin", func() {
			It("calls the unprivileged command creator to generate a destroy command", func() {
				Expect(destroyErr).NotTo(HaveOccurred())
				Expect(fakeUnprivilegedCommandCreator.DestroyCommandCallCount()).To(Equal(1))

				_, handleArg := fakeUnprivilegedCommandCreator.DestroyCommandArgsForCall(0)
				Expect(handleArg).To(Equal(handle))
			})

			It("runs the unprivileged plugin command with the command runner", func() {
				Expect(destroyErr).NotTo(HaveOccurred())
				Expect(fakeCommandRunner.ExecutedCommands()).To(HaveLen(2))
				executedCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(executedCmd).To(Equal(unprivCmd))
			})

			Context("when running the unprivileged image plugin destroy fails", func() {
				BeforeEach(func() {
					fakeUnprivImagePluginStdout = "unpriv-image-plugin-exploded-due-to-oom"
					fakeUnprivImagePluginError = errors.New("unpriv-image-plugin-delete-failed")
				})

				It("returns the wrapped error and plugin stdout, with context", func() {
					str := fmt.Sprintf("running image plugin destroy: %s: %s",
						fakeUnprivImagePluginStdout, fakeUnprivImagePluginError)
					Expect(destroyErr).To(MatchError(str))
				})
			})

			Context("when the unpriviliged image plugin emits logs to stderr", func() {
				BeforeEach(func() {
					buffer := gbytes.NewBuffer()
					externalLogger := lager.NewLogger("external-plugin")
					externalLogger.RegisterSink(lager.NewWriterSink(buffer, lager.DEBUG))
					externalLogger.Error("error-message", errors.New("failed!"), lager.Data{"type": "error"})

					fakeUnprivImagePluginStderr = string(buffer.Contents())
				})

				It("relogs the log entries", func() {
					Expect(fakeLogger).To(glager.ContainSequence(
						glager.Error(
							errors.New("failed!"),
							glager.Message("image-plugin.image-plugin-destroy.external-plugin.error-message"),
							glager.Data("type", "error"),
						),
					))
				})
			})
		})

		Describe("Running the privileged plugin", func() {
			It("calls the privileged command creator to generate a destroy command", func() {
				Expect(destroyErr).NotTo(HaveOccurred())
				Expect(fakePrivilegedCommandCreator.DestroyCommandCallCount()).To(Equal(1))

				_, handleArg := fakePrivilegedCommandCreator.DestroyCommandArgsForCall(0)
				Expect(handleArg).To(Equal(handle))
			})

			It("runs the privileged plugin command with the command runner", func() {
				Expect(destroyErr).NotTo(HaveOccurred())
				Expect(fakeCommandRunner.ExecutedCommands()).To(HaveLen(2))
				executedCmd := fakeCommandRunner.ExecutedCommands()[1]

				Expect(executedCmd).To(Equal(privCmd))
			})

			Context("when running the privileged image plugin destroy fails", func() {
				BeforeEach(func() {
					fakePrivImagePluginStdout = "priv-image-plugin-exploded-due-to-oom"
					fakePrivImagePluginError = errors.New("priv-image-plugin-delete-failed")
				})

				It("returns the wrapped error and plugin stdout, with context", func() {
					str := fmt.Sprintf("running image plugin destroy: %s: %s",
						fakePrivImagePluginStdout, fakePrivImagePluginError)
					Expect(destroyErr).To(MatchError(str))
				})
			})

			Context("when the unpriviliged image plugin emits logs to stderr", func() {
				BeforeEach(func() {
					buffer := gbytes.NewBuffer()
					externalLogger := lager.NewLogger("external-plugin")
					externalLogger.RegisterSink(lager.NewWriterSink(buffer, lager.DEBUG))
					externalLogger.Error("error-message", errors.New("failed!"), lager.Data{"type": "error"})

					fakePrivImagePluginStderr = string(buffer.Contents())
				})

				It("relogs the log entries", func() {
					Expect(fakeLogger).To(glager.ContainSequence(
						glager.Error(
							errors.New("failed!"),
							glager.Message("image-plugin.image-plugin-destroy.external-plugin.error-message"),
							glager.Data("type", "error"),
						),
					))
				})
			})
		})

	})

	Describe("Metrics", func() {
		var (
			cmd *exec.Cmd

			handle string

			fakeImagePluginStdout string
			fakeImagePluginStderr string
			fakeImagePluginError  error

			diskStats  garden.ContainerDiskStat
			metricsErr error

			namespaced bool
		)

		BeforeEach(func() {
			cmd = exec.Command("unpriv-plugin", "metrics")
			fakeUnprivilegedCommandCreator.MetricsCommandReturns(cmd)
			fakePrivilegedCommandCreator.MetricsCommandReturns(cmd)

			handle = "test-handle"

			fakeImagePluginStdout = `{"disk_usage": {"total_bytes_used": 100, "exclusive_bytes_used": 200}}`
			fakeImagePluginStderr = ""
			fakeImagePluginError = nil

			namespaced = true //assume unprivileged by default

			metricsErr = nil
		})

		JustBeforeEach(func() {
			fakeCommandRunner.WhenRunning(
				fake_command_runner.CommandSpec{
					Path: cmd.Path,
				},
				func(cmd *exec.Cmd) error {
					cmd.Stdout.Write([]byte(fakeImagePluginStdout))
					cmd.Stderr.Write([]byte(fakeImagePluginStderr))
					return fakeImagePluginError
				},
			)

			diskStats, metricsErr = imagePlugin.Metrics(fakeLogger, handle, namespaced)
		})

		It("calls the unprivileged command creator to generate a metrics command", func() {
			Expect(metricsErr).NotTo(HaveOccurred())
			Expect(fakePrivilegedCommandCreator.MetricsCommandCallCount()).To(Equal(0))
			Expect(fakeUnprivilegedCommandCreator.MetricsCommandCallCount()).To(Equal(1))

			_, handleArg := fakeUnprivilegedCommandCreator.MetricsCommandArgsForCall(0)
			Expect(handleArg).To(Equal(handle))
		})

		Context("when the image plugin available", func() {
			BeforeEach(func() {
				fakeUnprivilegedCommandCreator.MetricsCommandReturns(nil)
			})

			It("returns an error", func() {
				Expect(metricsErr).To(MatchError("requested image plugin not available"))
			})
		})

		Context("when getting metrics for an privileged volume", func() {
			BeforeEach(func() {
				namespaced = false
			})

			It("calls the privileged command creator to generate a metrics command", func() {
				Expect(metricsErr).NotTo(HaveOccurred())
				Expect(fakePrivilegedCommandCreator.MetricsCommandCallCount()).To(Equal(1))
				Expect(fakeUnprivilegedCommandCreator.MetricsCommandCallCount()).To(Equal(0))

				_, handleArg := fakePrivilegedCommandCreator.MetricsCommandArgsForCall(0)
				Expect(handleArg).To(Equal(handle))
			})

			Context("when the image plugin available", func() {
				BeforeEach(func() {
					fakePrivilegedCommandCreator.MetricsCommandReturns(nil)
				})

				It("returns an error", func() {
					Expect(metricsErr).To(MatchError("requested image plugin not available"))
				})
			})
		})

		It("runs the plugin command with the command runner", func() {
			Expect(metricsErr).NotTo(HaveOccurred())
			Expect(fakeCommandRunner.ExecutedCommands()).To(HaveLen(1))
			executedCmd := fakeCommandRunner.ExecutedCommands()[0]

			Expect(executedCmd).To(Equal(cmd))
		})

		Context("when running the image plugin metrics fails", func() {
			BeforeEach(func() {
				fakeImagePluginStdout = "image-plugin-exploded-due-to-oom"
				fakeImagePluginError = errors.New("image-plugin-stats-failed")
			})

			It("returns the wrapped error and plugin stdout, with context", func() {
				str := fmt.Sprintf("running image plugin metrics: %s: %s",
					fakeImagePluginStdout, fakeImagePluginError)
				Expect(metricsErr).To(MatchError(str))
			})
		})

		It("parses the plugin stdout as disk stats", func() {
			Expect(diskStats.TotalBytesUsed).To(BeEquivalentTo(100))
			Expect(diskStats.ExclusiveBytesUsed).To(BeEquivalentTo(200))
		})

		Context("when the plugin returns nonsense stats", func() {
			BeforeEach(func() {
				fakeImagePluginStdout = "NONSENSE_JSON"
			})

			It("returns a sensible error containing the json", func() {
				Expect(metricsErr).To(MatchError(ContainSubstring("parsing stats: NONSENSE_JSON")))
			})
		})

		Context("when the image plugin emits logs to stderr", func() {
			BeforeEach(func() {
				buffer := gbytes.NewBuffer()
				externalLogger := lager.NewLogger("external-plugin")
				externalLogger.RegisterSink(lager.NewWriterSink(buffer, lager.DEBUG))
				externalLogger.Error("error-message", errors.New("failed!"), lager.Data{"type": "error"})

				fakeImagePluginStderr = string(buffer.Contents())
			})

			It("relogs the log entries", func() {
				Expect(fakeLogger).To(glager.ContainSequence(
					glager.Error(
						errors.New("failed!"),
						glager.Message("image-plugin.image-plugin-metrics.external-plugin.error-message"),
						glager.Data("type", "error"),
					),
				))
			})
		})
	})

	Describe("Capacity", func() {
		var (
			cmd *exec.Cmd

			fakeImagePluginStdout string
			fakeImagePluginStderr string
			fakeImagePluginError  error

			capacity    uint64
			capacityErr error
		)

		BeforeEach(func() {
			cmd = exec.Command("unpriv-plugin", "capacity")
			fakeUnprivilegedCommandCreator.CapacityCommandReturns(cmd)

			fakeImagePluginStdout = `{"capacity": 888}`
			fakeImagePluginStderr = ""
			fakeImagePluginError = nil

			capacityErr = nil
		})

		JustBeforeEach(func() {
			fakeCommandRunner.WhenRunning(
				fake_command_runner.CommandSpec{
					Path: cmd.Path,
					Args: []string{"capacity"},
				},
				func(cmd *exec.Cmd) error {
					cmd.Stdout.Write([]byte(fakeImagePluginStdout))
					cmd.Stderr.Write([]byte(fakeImagePluginStderr))
					return fakeImagePluginError
				},
			)

			capacity, capacityErr = imagePlugin.Capacity(fakeLogger)
		})

		It("runs the plugin command with the command runner", func() {
			Expect(capacityErr).NotTo(HaveOccurred())
			Expect(capacity).To(Equal(uint64(888)))
		})

		Context("when running the image plugin fails", func() {
			BeforeEach(func() {
				fakeImagePluginStdout = "image-plugin-exploded-due-to-oom"
				fakeImagePluginError = errors.New("image-plugin-stats-failed")
			})

			It("returns the wrapped error and plugin stdout, with context", func() {
				str := fmt.Sprintf("running image plugin capacity: %s: %s",
					fakeImagePluginStdout, fakeImagePluginError)
				Expect(capacityErr).To(MatchError(str))
			})
		})

		Context("when the plugin returns nonsense stats", func() {
			BeforeEach(func() {
				fakeImagePluginStdout = "NONSENSE_JSON"
			})

			It("returns a sensible error containing the json", func() {
				Expect(capacityErr).To(MatchError(ContainSubstring("parsing capacity: NONSENSE_JSON")))
			})
		})

		Context("when the image plugin emits logs to stderr", func() {
			BeforeEach(func() {
				buffer := gbytes.NewBuffer()
				externalLogger := lager.NewLogger("external-plugin")
				externalLogger.RegisterSink(lager.NewWriterSink(buffer, lager.DEBUG))
				externalLogger.Error("error-message", errors.New("failed!"), lager.Data{"type": "error"})

				fakeImagePluginStderr = string(buffer.Contents())
			})

			It("relogs the log entries", func() {
				Expect(fakeLogger).To(glager.ContainSequence(
					glager.Error(
						errors.New("failed!"),
						glager.Message("image-plugin.image-plugin-capacity.external-plugin.error-message"),
						glager.Data("type", "error"),
					),
				))
			})
		})
	})
})

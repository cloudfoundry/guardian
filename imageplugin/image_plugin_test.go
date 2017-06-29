package imageplugin_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden-shed/rootfs_spec"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/imageplugin"
	fakes "code.cloudfoundry.org/guardian/imageplugin/imagepluginfakes"
	"code.cloudfoundry.org/lager"
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
		fakeCommandRunner              *fake_command_runner.FakeCommandRunner

		fakeLogger lager.Logger

		defaultRootfs string
	)

	BeforeEach(func() {
		fakeUnprivilegedCommandCreator = new(fakes.FakeCommandCreator)
		fakePrivilegedCommandCreator = new(fakes.FakeCommandCreator)
		fakeCommandRunner = fake_command_runner.New()

		fakeLogger = glager.NewLogger("image-plugin")

		defaultRootfs = "/default-rootfs"
	})

	JustBeforeEach(func() {
		imagePlugin = imageplugin.ImagePlugin{
			UnprivilegedCommandCreator: fakeUnprivilegedCommandCreator,
			PrivilegedCommandCreator:   fakePrivilegedCommandCreator,
			CommandRunner:              fakeCommandRunner,
			DefaultRootfs:              defaultRootfs,
		}
	})

	Describe("Create", func() {
		var (
			cmd *exec.Cmd

			handle             string
			rootfsProviderSpec rootfs_spec.Spec
			rootfs             string
			namespaced         bool

			// fakeImagePluginStdout will override createOutputs if set
			createOutputs         gardener.DesiredImageSpec
			fakeImagePluginStdout string
			fakeImagePluginStderr string
			fakeImagePluginError  error

			createImageSpec gardener.DesiredImageSpec
			createErr       error
		)

		BeforeEach(func() {
			cmd = exec.Command("unpriv-plugin", "create")
			fakeUnprivilegedCommandCreator.CreateCommandReturns(cmd, nil)
			fakePrivilegedCommandCreator.CreateCommandReturns(cmd, nil)

			handle = "test-handle"
			rootfs = "docker:///busybox"
			namespaced = true //assume unprivileged by default

			createOutputs = gardener.DesiredImageSpec{
				RootFS: "/image-rootfs/rootfs",
			}
			fakeImagePluginStdout = ""
			fakeImagePluginStderr = ""
			fakeImagePluginError = nil

			createImageSpec = gardener.DesiredImageSpec{}
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
						b, err := json.Marshal(createOutputs)
						Expect(err).NotTo(HaveOccurred())
						fakeImagePluginStdout = string(b)
					}

					cmd.Stdout.Write([]byte(fakeImagePluginStdout))
					return fakeImagePluginError
				},
			)

			rootfsURL, err := url.Parse(rootfs)
			Expect(err).NotTo(HaveOccurred())
			rootfsProviderSpec = rootfs_spec.Spec{RootFS: rootfsURL, Namespaced: namespaced}
			createImageSpec, createErr = imagePlugin.Create(fakeLogger, handle, rootfsProviderSpec)
		})

		It("calls the unprivileged command creator to generate a create command", func() {
			Expect(createErr).NotTo(HaveOccurred())
			Expect(fakeUnprivilegedCommandCreator.CreateCommandCallCount()).To(Equal(1))
			Expect(fakePrivilegedCommandCreator.CreateCommandCallCount()).To(Equal(0))

			_, handleArg, specArg := fakeUnprivilegedCommandCreator.CreateCommandArgsForCall(0)
			Expect(handleArg).To(Equal(handle))
			Expect(specArg).To(Equal(rootfsProviderSpec))
		})

		Context("when the unprivileged command creator returns an error", func() {
			BeforeEach(func() {
				fakeUnprivilegedCommandCreator.CreateCommandReturns(nil, errors.New("explosion"))
			})

			It("returns that error", func() {
				Expect(createErr).To(MatchError("creating create command: explosion"))
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
			Expect(createImageSpec.RootFS).To(Equal("/image-rootfs/rootfs"))
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
				Expect(createImageSpec.Image.Config.Env).To(BeEmpty())
			})
		})

		Context("when there is image config defined", func() {
			BeforeEach(func() {
				createOutputs.Image = gardener.Image{
					Config: gardener.ImageConfig{
						Env: []string{
							"MY_VAR=set",
							"MY_SECOND_VAR=also_set",
						},
					},
				}
			})

			It("returns the list of env variables to set", func() {
				Expect(createImageSpec.Image.Config.Env).To(ConsistOf([]string{"MY_VAR=set", "MY_SECOND_VAR=also_set"}))
			})
		})

		Context("when there are mounts defined", func() {
			BeforeEach(func() {
				createOutputs.Mounts = []specs.Mount{
					{
						Source:      "src",
						Destination: "dest",
						Options:     []string{"bind"},
						Type:        "bind",
					},
				}
			})

			It("returns the list of mounts to configure", func() {
				Expect(createImageSpec.Mounts).To(Equal(createOutputs.Mounts))
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

		Context("when destroying an unprivileged volume", func() {
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
})

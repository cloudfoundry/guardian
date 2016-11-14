package imageplugin_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/garden-shed/rootfs_provider"
	"code.cloudfoundry.org/guardian/imageplugin"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("ExternalImageManager", func() {
	var (
		fakeCommandRunner    *fake_command_runner.FakeCommandRunner
		logger               *lagertest.TestLogger
		externalImageManager *imageplugin.ExternalImageManager
		baseImage            *url.URL
		idMappings           []specs.LinuxIDMapping
		defaultBaseImage     *url.URL
		fakeCmdRunnerStdout  string
		fakeCmdRunnerStderr  string
		fakeCmdRunnerErr     error
	)

	BeforeEach(func() {
		fakeCmdRunnerStdout = ""
		fakeCmdRunnerStderr = ""
		fakeCmdRunnerErr = nil

		logger = lagertest.NewTestLogger("external-image-manager")
		fakeCommandRunner = fake_command_runner.New()

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

		var err error
		defaultBaseImage, err = url.Parse("/default/image")
		Expect(err).ToNot(HaveOccurred())
		externalImageManager = imageplugin.New("/external-image-manager-bin", fakeCommandRunner, defaultBaseImage, idMappings)

		baseImage, err = url.Parse("/hello/image")
		Expect(err).ToNot(HaveOccurred())

		fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
			Path: "/external-image-manager-bin",
		}, func(cmd *exec.Cmd) error {
			if cmd.Stdout != nil {
				cmd.Stdout.Write([]byte(fakeCmdRunnerStdout))
			}
			if cmd.Stderr != nil {
				cmd.Stderr.Write([]byte(fakeCmdRunnerStderr))
			}

			return fakeCmdRunnerErr
		})
	})

	Describe("Create", func() {
		BeforeEach(func() {
			fakeCmdRunnerStdout = "/this-is/your\n"
		})

		It("uses the correct external-image-manager binary", func() {
			_, _, err := externalImageManager.Create(
				logger, "hello", rootfs_provider.Spec{
					RootFS: baseImage,
				},
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
			imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

			Expect(imageManagerCmd.Path).To(Equal("/external-image-manager-bin"))
		})

		It("returns the env variables defined in the image configuration", func() {
			imagePath, err := ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
			fakeCmdRunnerStdout = imagePath

			imageConfig := imageplugin.Image{
				Config: imageplugin.ImageConfig{
					Env: []string{"HELLO=there", "PATH=/my-path/bin"},
				},
			}

			imageConfigFile, err := os.Create(filepath.Join(imagePath, "image.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(json.NewEncoder(imageConfigFile).Encode(imageConfig)).To(Succeed())

			_, envVariables, err := externalImageManager.Create(
				logger, "hello", rootfs_provider.Spec{
					RootFS: baseImage,
				},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(envVariables).To(ConsistOf([]string{"HELLO=there", "PATH=/my-path/bin"}))
		})

		Context("when the image configuration file is inaccessible", func() {
			It("returns an error", func() {
				imagePath, err := ioutil.TempDir("", "")
				Expect(err).NotTo(HaveOccurred())
				fakeCmdRunnerStdout = imagePath
				Expect(ioutil.WriteFile(filepath.Join(imagePath, "image.json"), []byte("{}"), 0000)).To(Succeed())

				_, _, err = externalImageManager.Create(
					logger, "hello", rootfs_provider.Spec{
						RootFS: baseImage,
					},
				)
				Expect(err).To(MatchError(ContainSubstring("could not open image configuration")))
			})
		})

		Context("when the image configuration is not defined", func() {
			It("returns an empty list of environment variables", func() {
				_, envVariables, err := externalImageManager.Create(
					logger, "hello", rootfs_provider.Spec{
						RootFS: baseImage,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(envVariables).To(BeEmpty())
			})
		})

		Context("when the image configuration is not valid json", func() {
			It("returns an error", func() {
				imagePath, err := ioutil.TempDir("", "")
				Expect(err).NotTo(HaveOccurred())
				fakeCmdRunnerStdout = imagePath
				Expect(ioutil.WriteFile(filepath.Join(imagePath, "image.json"), []byte("what-image: is this: no"), 0666)).To(Succeed())

				_, _, err = externalImageManager.Create(
					logger, "hello", rootfs_provider.Spec{
						RootFS: baseImage,
					},
				)
				Expect(err).To(MatchError(ContainSubstring("parsing image config")))
			})
		})

		Describe("external-image-manager parameters", func() {
			It("uses the correct external-image-manager create command", func() {
				_, _, err := externalImageManager.Create(
					logger, "hello", rootfs_provider.Spec{
						RootFS: baseImage,
					},
				)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(imageManagerCmd.Args[1]).To(Equal("create"))
			})

			It("sets the correct image input to external-image-manager", func() {
				_, _, err := externalImageManager.Create(
					logger, "hello", rootfs_provider.Spec{
						RootFS: baseImage,
					},
				)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(imageManagerCmd.Args[len(imageManagerCmd.Args)-2]).To(Equal("/hello/image"))
			})

			It("sets the correct id to external-image-manager", func() {
				_, _, err := externalImageManager.Create(
					logger, "hello", rootfs_provider.Spec{
						RootFS: baseImage,
					},
				)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(imageManagerCmd.Args[len(imageManagerCmd.Args)-1]).To(Equal("hello"))
			})

			Context("when namespaced is true", func() {
				It("passes the correct uid and gid mappings to the external-image-manager", func() {
					_, _, err := externalImageManager.Create(
						logger, "hello", rootfs_provider.Spec{
							RootFS:     baseImage,
							Namespaced: true,
						},
					)
					Expect(err).ToNot(HaveOccurred())

					Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
					imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

					firstMap := fmt.Sprintf("%d:%d:%d", idMappings[0].ContainerID, idMappings[0].HostID, idMappings[0].Size)
					secondMap := fmt.Sprintf("%d:%d:%d", idMappings[1].ContainerID, idMappings[1].HostID, idMappings[1].Size)

					Expect(imageManagerCmd.Args[2:10]).To(Equal([]string{
						"--uid-mapping", firstMap,
						"--gid-mapping", firstMap,
						"--uid-mapping", secondMap,
						"--gid-mapping", secondMap,
					}))
				})

				It("runs the external-image-manager as the container root user", func() {
					_, _, err := externalImageManager.Create(
						logger, "hello", rootfs_provider.Spec{
							RootFS:     baseImage,
							Namespaced: true,
						},
					)
					Expect(err).ToNot(HaveOccurred())

					Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
					imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

					Expect(imageManagerCmd.SysProcAttr.Credential.Uid).To(Equal(idMappings[0].HostID))
					Expect(imageManagerCmd.SysProcAttr.Credential.Gid).To(Equal(idMappings[0].HostID))
				})
			})

			Context("when namespaced is false", func() {
				It("does not pass any uid and gid mappings to the external-image-manager", func() {
					_, _, err := externalImageManager.Create(
						logger, "hello", rootfs_provider.Spec{
							RootFS: baseImage,
						},
					)
					Expect(err).ToNot(HaveOccurred())

					Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
					imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

					Expect(imageManagerCmd.Args).NotTo(ContainElement("--uid-mapping"))
					Expect(imageManagerCmd.Args).NotTo(ContainElement("--gid-mapping"))
				})
			})

			Context("when a disk quota is provided in the spec", func() {
				It("passes the quota to the external-image-manager", func() {
					_, _, err := externalImageManager.Create(
						logger, "hello", rootfs_provider.Spec{
							RootFS:    baseImage,
							QuotaSize: 1024,
						},
					)
					Expect(err).ToNot(HaveOccurred())

					Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
					imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

					Expect(imageManagerCmd.Args[2]).To(Equal("--disk-limit-size-bytes"))
					Expect(imageManagerCmd.Args[3]).To(Equal("1024"))
				})
			})
		})

		It("returns rootfs location", func() {
			returnedRootFS, _, err := externalImageManager.Create(
				logger, "hello", rootfs_provider.Spec{
					RootFS: baseImage,
				},
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(returnedRootFS).To(Equal("/this-is/your/rootfs"))
		})

		Context("when the command fails", func() {
			BeforeEach(func() {
				fakeCmdRunnerStdout = "could not find drax"
				fakeCmdRunnerStderr = "btrfs doesn't like you"
				fakeCmdRunnerErr = errors.New("external-image-manager failure")
			})

			It("returns an error", func() {
				_, _, err := externalImageManager.Create(
					logger, "hello", rootfs_provider.Spec{
						RootFS: baseImage,
					},
				)

				Expect(err).To(MatchError(ContainSubstring("external image manager create failed")))
				Expect(err).To(MatchError(ContainSubstring("external-image-manager failure")))
				Expect(err).To(MatchError(ContainSubstring("could not find drax")))
			})

			It("returns the external-image-manager error output in the error", func() {
				_, _, err := externalImageManager.Create(
					logger, "hello", rootfs_provider.Spec{
						RootFS: baseImage,
					},
				)
				Expect(err).To(HaveOccurred())

				Expect(logger).To(gbytes.Say("btrfs doesn't like you"))
			})
		})

		Context("when a RootFS is not provided in the rootfs_provider.Spec", func() {
			It("passes the default rootfs to the external-image-manager", func() {
				_, _, err := externalImageManager.Create(
					logger, "hello", rootfs_provider.Spec{},
				)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(imageManagerCmd.Args[len(imageManagerCmd.Args)-2]).To(Equal(defaultBaseImage.String()))
			})
		})
	})

	Describe("Destroy", func() {
		It("uses the correct external-image-manager binary", func() {
			Expect(externalImageManager.Destroy(
				logger, "hello", "/store/0/images/123/rootfs",
			)).To(Succeed())
			Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
			imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

			Expect(imageManagerCmd.Path).To(Equal("/external-image-manager-bin"))
		})

		Describe("external-image-manager parameters", func() {
			It("uses the correct external-image-manager delete command", func() {
				Expect(externalImageManager.Destroy(
					logger, "hello", "/store/0/images/123/rootfs",
				)).To(Succeed())
				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(imageManagerCmd.Args[1]).To(Equal("delete"))
			})

			It("passes the correct image path to delete to/ the external-image-manager", func() {
				Expect(externalImageManager.Destroy(
					logger, "hello", "/store/0/images/123/rootfs",
				)).To(Succeed())
				Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
				imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(imageManagerCmd.Args[len(imageManagerCmd.Args)-1]).To(Equal("/store/0/images/123"))
			})
		})

		Context("when the command fails", func() {
			BeforeEach(func() {
				fakeCmdRunnerStdout = "could not find drax"
				fakeCmdRunnerStderr = "btrfs doesn't like you"
				fakeCmdRunnerErr = errors.New("external-image-manager failure")
			})

			It("returns an error", func() {
				err := externalImageManager.Destroy(logger, "hello", "/store/0/images/123/rootfs")

				Expect(err).To(MatchError(ContainSubstring("external image manager destroy failed")))
				Expect(err).To(MatchError(ContainSubstring("external-image-manager failure")))
				Expect(err).To(MatchError(ContainSubstring("could not find drax")))
			})

			It("returns the external-image-manager error output in the error", func() {
				Expect(externalImageManager.Destroy(
					logger, "hello", "/store/0/images/123/rootfs",
				)).NotTo(Succeed())

				Expect(logger).To(gbytes.Say("btrfs doesn't like you"))
			})
		})
	})

	Describe("GC", func() {
		var imageManagerCmd *exec.Cmd

		It("uses the correct external-image-manager binary", func() {
			Expect(externalImageManager.GC(logger)).NotTo(HaveOccurred())
			imageManagerCmd = fakeCommandRunner.ExecutedCommands()[0]
			Expect(imageManagerCmd.Path).To(Equal("/external-image-manager-bin"))
		})

		It("calls external image clean command", func() {
			Expect(externalImageManager.GC(logger)).NotTo(HaveOccurred())
			imageManagerCmd = fakeCommandRunner.ExecutedCommands()[0]
			Expect(imageManagerCmd.Args[1]).To(Equal("clean"))
		})

		Context("when the command fails", func() {
			BeforeEach(func() {
				fakeCmdRunnerErr = errors.New("external-image-manager failure")
				fakeCmdRunnerStderr = "btrfs doesn't like you"
				fakeCmdRunnerStdout = "could not find drax"
			})

			It("returns an error", func() {
				err := externalImageManager.GC(logger)
				Expect(err).To(MatchError(ContainSubstring("external image manager clean failed")))
				Expect(err).To(MatchError(ContainSubstring("external-image-manager failure")))
				Expect(err).To(MatchError(ContainSubstring("could not find drax")))
			})

			It("forwards the external-image-manager error output", func() {
				externalImageManager.GC(logger)
				Expect(logger).To(gbytes.Say("btrfs doesn't like you"))
			})
		})
	})

	Describe("Metrics", func() {
		BeforeEach(func() {
			fakeCmdRunnerErr = nil
			fakeCmdRunnerStdout = `{"disk_usage": {"total_bytes_used": 1000, "exclusive_bytes_used": 2000}}`
			fakeCmdRunnerStderr = ""
		})

		It("uses the correct external-image-manager binary", func() {
			_, err := externalImageManager.Metrics(logger, "", "/store/0/bundles/123/rootfs")
			Expect(err).ToNot(HaveOccurred())
			Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
			imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

			Expect(imageManagerCmd.Path).To(Equal("/external-image-manager-bin"))
		})

		It("calls external-image-manager with the correct arguments", func() {
			_, err := externalImageManager.Metrics(logger, "", "/store/0/bundles/123/rootfs")
			Expect(err).ToNot(HaveOccurred())
			Expect(len(fakeCommandRunner.ExecutedCommands())).To(Equal(1))
			imageManagerCmd := fakeCommandRunner.ExecutedCommands()[0]

			Expect(imageManagerCmd.Args[1]).To(Equal("metrics"))
			Expect(imageManagerCmd.Args[2]).To(Equal("/store/0/bundles/123"))
		})

		It("returns the correct ContainerDiskStat", func() {
			stats, err := externalImageManager.Metrics(logger, "", "/store/0/bundles/123/rootfs")
			Expect(err).ToNot(HaveOccurred())

			Expect(stats.TotalBytesUsed).To(Equal(uint64(1000)))
			Expect(stats.ExclusiveBytesUsed).To(Equal(uint64(2000)))
		})

		Context("when the image plugin fails", func() {
			It("returns an error", func() {
				fakeCmdRunnerStdout = "could not find drax"
				fakeCmdRunnerErr = errors.New("failed to get metrics")
				_, err := externalImageManager.Metrics(logger, "", "/store/0/bundles/123/rootfs")
				Expect(err).To(MatchError(ContainSubstring("failed to get metrics")))
				Expect(err).To(MatchError(ContainSubstring("could not find drax")))
			})
		})

		Context("when the image plugin output parsing fails", func() {
			It("returns an error", func() {
				fakeCmdRunnerStdout = `{"silly" "json":"formating}"}}"`
				_, err := externalImageManager.Metrics(logger, "", "/store/0/bundles/123/rootfs")
				Expect(err).To(MatchError(ContainSubstring("parsing metrics")))
			})
		})
	})
})

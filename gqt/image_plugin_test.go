package gqt_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Image Plugin", func() {

	var (
		tmpDir                            string
		client                            *runner.RunningGarden
		containerDestructionShouldSucceed bool
	)

	BeforeEach(func() {
		containerDestructionShouldSucceed = true
		var err error
		tmpDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Chmod(tmpDir, 0777)).To(Succeed())
	})

	JustBeforeEach(func() {
		client = runner.Start(config)
	})

	AfterEach(func() {
		err := client.DestroyAndStop()
		if containerDestructionShouldSucceed {
			Expect(err).NotTo(HaveOccurred())
		} else {
			Expect(err).NotTo(BeAssignableToTypeOf(runner.ErrGardenStop{}))
		}
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	Context("when only an unprivileged image plugin is provided", func() {
		BeforeEach(func() {
			config.ImagePluginBin = binaries.ImagePlugin
			config.ImagePluginExtraArgs = []string{
				"\"--rootfs-path\"",
				tmpDir,
				"\"--args-path\"",
				filepath.Join(tmpDir, "args"),
			}
		})

		Context("and an unprivileged container is successfully created", func() {
			var (
				containerSpec garden.ContainerSpec
				container     garden.Container
				handle        string
				tmpMountDir   string
			)

			BeforeEach(func() {
				tmpMountDir, err := ioutil.TempDir("", "")
				Expect(err).NotTo(HaveOccurred())

				containerSpec = garden.ContainerSpec{
					BindMounts: []garden.BindMount{
						garden.BindMount{
							SrcPath: tmpMountDir,
							DstPath: "/user-provided-bind-mount",
							Mode:    garden.BindMountModeRO,
						},
					},
				}
				config.ImagePluginExtraArgs = append(
					config.ImagePluginExtraArgs,
					"\"--create-whoami-path\"",
					filepath.Join(tmpDir, "create-whoami"),
				)
			})

			JustBeforeEach(func() {
				var err error
				container, err = client.Create(containerSpec)
				Expect(err).NotTo(HaveOccurred())
				handle = container.Handle()
			})

			AfterEach(func() {
				Expect(os.RemoveAll(tmpMountDir)).To(Succeed())
			})

			It("executes the plugin, passing the correct args", func() {
				pluginArgsBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "args"))
				Expect(err).ToNot(HaveOccurred())

				pluginArgs := strings.Split(string(pluginArgsBytes), " ")
				Expect(pluginArgs).To(Equal([]string{
					binaries.ImagePlugin,
					"--rootfs-path", tmpDir,
					"--args-path", filepath.Join(tmpDir, "args"),
					"--create-whoami-path", filepath.Join(tmpDir, "create-whoami"),
					"create",
					defaultTestRootFS,
					handle,
				}))
			})

			It("executes the plugin as root", func() {
				whoamiBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "create-whoami"))
				Expect(err).NotTo(HaveOccurred())

				Expect(string(whoamiBytes)).To(ContainSubstring("0 - 0"))
			})

			Context("when there are env vars", func() {
				BeforeEach(func() {
					image := gardener.Image{
						Config: gardener.ImageConfig{
							Env: []string{
								"MY_VAR=set",
								"MY_SECOND_VAR=also_set",
							},
						},
					}
					imageJson, err := json.Marshal(image)
					Expect(err).NotTo(HaveOccurred())

					config.ImagePluginExtraArgs = append(
						config.ImagePluginExtraArgs,
						"\"--image-json\"",
						string(imageJson),
					)

					gardenDefaultRootfs := defaultTestRootFS
					Expect(copyFile(filepath.Join(gardenDefaultRootfs, "bin", "env"),
						filepath.Join(tmpDir, "env"))).To(Succeed())
				})

				It("loads the image.json env variables", func() {
					buffer := gbytes.NewBuffer()
					process, err := container.Run(garden.ProcessSpec{
						Path: "/env",
						Dir:  "/",
					}, garden.ProcessIO{Stdout: buffer, Stderr: buffer})
					Expect(err).NotTo(HaveOccurred())
					exitCode, err := process.Wait()
					Expect(err).NotTo(HaveOccurred())
					Expect(exitCode).To(BeZero())

					Eventually(buffer).Should(gbytes.Say("MY_VAR=set"))
					Eventually(buffer).Should(gbytes.Say("MY_SECOND_VAR=also_set"))
				})
			})

			Context("when there are mounts", func() {
				var mountedDir string
				fileName := "mnted-file"
				fileContent := "mnted-file-content"

				BeforeEach(func() {
					var err error
					mountedDir, err = ioutil.TempDir("", "bind-mount")
					Expect(err).NotTo(HaveOccurred())
					Expect(os.Chmod(mountedDir, 0777)).To(Succeed())
					Expect(ioutil.WriteFile(filepath.Join(mountedDir, fileName), []byte(fileContent), 0644)).To(Succeed())

					mounts := []specs.Mount{
						{
							Type:        "bind",
							Options:     []string{"bind"},
							Source:      mountedDir,
							Destination: "/image-plugin-bind-mount",
						},
					}
					mountsJson, err := json.Marshal(mounts)
					Expect(err).NotTo(HaveOccurred())

					config.ImagePluginExtraArgs = append(
						config.ImagePluginExtraArgs,
						"\"--mounts-json\"",
						string(mountsJson),
					)

					gardenDefaultRootfs := defaultTestRootFS
					Expect(copyFile(filepath.Join(gardenDefaultRootfs, "bin", "cat"),
						filepath.Join(tmpDir, "cat"))).To(Succeed())
				})

				AfterEach(func() {
					Expect(os.RemoveAll(mountedDir)).To(Succeed())
				})

				It("mounts the directories from the image plugin response", func() {
					var stdout bytes.Buffer
					process, err := container.Run(garden.ProcessSpec{
						Path: "/cat",
						Args: []string{filepath.Join("/image-plugin-bind-mount", fileName)},
					}, garden.ProcessIO{
						Stdout: io.MultiWriter(&stdout, GinkgoWriter),
						Stderr: GinkgoWriter,
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(process.Wait()).To(Equal(0))
					Expect(stdout.String()).To(Equal(fileContent))
				})

				It("ensures that the mounts from the image plugin response are first in config.json", func() {
					config, err := ioutil.ReadFile(filepath.Join(client.DepotDir, container.Handle(), "config.json"))
					Expect(err).NotTo(HaveOccurred())

					var spec specs.Spec
					Expect(json.Unmarshal(config, &spec)).To(Succeed())

					Expect(spec.Mounts[0].Destination).To(Equal("/image-plugin-bind-mount"))
				})
			})

			Context("when rootfs is not specified", func() {
				BeforeEach(func() {
					containerSpec.RootFSPath = ""
				})

				It("uses the default rootfs", func() {
					pluginArgsBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "args"))
					Expect(err).ToNot(HaveOccurred())

					Expect(string(pluginArgsBytes)).To(ContainSubstring(defaultTestRootFS))
				})
			})

			Context("when passing a tagged docker image as the RootFSPath", func() {
				BeforeEach(func() {
					containerSpec.RootFSPath = "docker:///busybox#1.26.1"
				})

				It("replaces the # with :", func() {
					pluginArgsBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "args"))
					Expect(err).ToNot(HaveOccurred())

					Expect(string(pluginArgsBytes)).To(ContainSubstring("docker:///busybox:1.26.1"))
				})
			})

			Context("when passing an Image URI", func() {
				BeforeEach(func() {
					containerSpec.Image = garden.ImageRef{URI: "/some/fake/rootfs"}
				})

				It("executes the plugin, passing the Image URI", func() {
					pluginArgsBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "args"))
					Expect(err).ToNot(HaveOccurred())

					Expect(string(pluginArgsBytes)).To(ContainSubstring("/some/fake/rootfs"))
				})
			})

			Context("when specifying a quota", func() {
				BeforeEach(func() {
					containerSpec.Limits.Disk.ByteHard = 100000
				})

				It("calls the image plugin setting the quota", func() {
					pluginArgsBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "args"))
					Expect(err).ToNot(HaveOccurred())

					Expect(string(pluginArgsBytes)).To(ContainSubstring("--disk-limit-size-bytes 100000"))
				})

				Context("when the quota is total", func() {
					BeforeEach(func() {
						containerSpec.Limits.Disk.Scope = garden.DiskLimitScopeTotal
					})

					It("calls the image plugin without the exclusive flag", func() {
						pluginArgsBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "args"))
						Expect(err).ToNot(HaveOccurred())

						Expect(string(pluginArgsBytes)).NotTo(ContainSubstring("--exclude-image-from-quota"))
					})
				})

				Context("when the quota is exclusive", func() {
					BeforeEach(func() {
						containerSpec.Limits.Disk.Scope = garden.DiskLimitScopeExclusive
					})

					It("calls the image plugin setting the exclusive flag", func() {
						pluginArgsBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "args"))
						Expect(err).ToNot(HaveOccurred())

						Expect(string(pluginArgsBytes)).To(ContainSubstring("--exclude-image-from-quota"))
					})
				})

				Context("when the plugin logs to stderr", func() {
					BeforeEach(func() {
						config.ImagePluginExtraArgs = append(
							config.ImagePluginExtraArgs,
							"\"--create-log-content\"",
							"CREATE-FAKE-LOG-LINE",
						)
					})

					It("relogs the plugin's stderr to the garden logs", func() {
						Eventually(client).Should(gbytes.Say("CREATE-FAKE-LOG-LINE"))
					})
				})
			})

			Context("and metrics are collected on that container", func() {
				var metrics garden.Metrics

				BeforeEach(func() {
					config.ImagePluginExtraArgs = append(
						config.ImagePluginExtraArgs,
						"\"--metrics-whoami-path\"",
						filepath.Join(tmpDir, "metrics-whoami"),
						"\"--metrics-output\"",
						`{"disk_usage": {"total_bytes_used": 1000, "exclusive_bytes_used": 2000}}`,
					)
				})

				JustBeforeEach(func() {
					var err error
					metrics, err = container.Metrics()
					Expect(err).NotTo(HaveOccurred())
				})

				It("executes the plugin, passing the correct args", func() {
					pluginArgsBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "args"))
					Expect(err).ToNot(HaveOccurred())

					pluginArgs := strings.Split(string(pluginArgsBytes), " ")
					Expect(pluginArgs).To(Equal([]string{
						binaries.ImagePlugin,
						"--rootfs-path", tmpDir,
						"--args-path", filepath.Join(tmpDir, "args"),
						"--create-whoami-path", filepath.Join(tmpDir, "create-whoami"),
						"--metrics-whoami-path", filepath.Join(tmpDir, "metrics-whoami"),
						"--metrics-output",
						"{\"disk_usage\":",
						"{\"total_bytes_used\":",
						"1000,",
						"\"exclusive_bytes_used\":",
						"2000}}",
						"stats",
						handle,
					}))
				})

				It("executes the plugin as root", func() {
					whoamiBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "metrics-whoami"))
					Expect(err).NotTo(HaveOccurred())

					Expect(string(whoamiBytes)).To(ContainSubstring("0 - 0"))
				})

				It("returns the plugin stdout as disk stats", func() {
					Expect(metrics.DiskStat.TotalBytesUsed).To(BeEquivalentTo(1000))
					Expect(metrics.DiskStat.ExclusiveBytesUsed).To(BeEquivalentTo(2000))
				})

				Context("when the plugin logs to stderr", func() {
					BeforeEach(func() {
						config.ImagePluginExtraArgs = append(
							config.ImagePluginExtraArgs,
							"\"--metrics-log-content\"",
							"METRICS-FAKE-LOG-LINE",
						)
					})

					It("relogs the plugin's stderr to the garden logs", func() {
						Eventually(client).Should(gbytes.Say("METRICS-FAKE-LOG-LINE"))
					})
				})
			})

			Context("but the plugin returns nonsense stats", func() {
				BeforeEach(func() {
					config.ImagePluginExtraArgs = append(
						config.ImagePluginExtraArgs,
						"\"--metrics-output\"",
						"NONSENSE_JSON",
					)
				})

				It("returns a sensible error containing the json", func() {
					_, err := container.Metrics()
					Expect(err).To(MatchError(ContainSubstring("parsing stats: NONSENSE_JSON")))
				})
			})

			Context("but the plugin fails when collecting metrics", func() {
				BeforeEach(func() {
					config.ImagePluginExtraArgs = append(
						config.ImagePluginExtraArgs,
						"\"--fail-on\"",
						"metrics",
					)
				})

				It("returns the plugin's stdout in a useful error", func() {
					_, err := container.Metrics()
					Expect(err).To(MatchError(ContainSubstring("running image plugin metrics: metrics failed")))
				})
			})

			Context("and that container is destroyed", func() {
				BeforeEach(func() {
					config.ImagePluginExtraArgs = append(
						config.ImagePluginExtraArgs,
						"\"--destroy-whoami-path\"",
						filepath.Join(tmpDir, "destroy-whoami"),
					)
				})

				JustBeforeEach(func() {
					Expect(client.Destroy(container.Handle())).Should(Succeed())
				})

				It("executes the plugin, passing the correct args", func() {
					pluginArgsBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "args"))
					Expect(err).ToNot(HaveOccurred())

					pluginArgs := strings.Split(string(pluginArgsBytes), " ")
					Expect(pluginArgs).To(Equal([]string{
						binaries.ImagePlugin,
						"--rootfs-path", tmpDir,
						"--args-path", filepath.Join(tmpDir, "args"),
						"--create-whoami-path", filepath.Join(tmpDir, "create-whoami"),
						"--destroy-whoami-path", filepath.Join(tmpDir, "destroy-whoami"),
						"delete",
						handle,
					}))
				})

				It("executes the plugin as root", func() {
					whoamiBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "destroy-whoami"))
					Expect(err).NotTo(HaveOccurred())

					Expect(string(whoamiBytes)).To(ContainSubstring("0 - 0"))
				})

				Context("when the plugin logs to stderr", func() {
					BeforeEach(func() {
						config.ImagePluginExtraArgs = append(
							config.ImagePluginExtraArgs,
							"\"--destroy-log-content\"",
							"DESTROY-FAKE-LOG-LINE",
						)
					})

					It("relogs the plugin's stderr to the garden logs", func() {
						Eventually(client).Should(gbytes.Say("DESTROY-FAKE-LOG-LINE"))
					})
				})
			})

			Context("but the plugin fails on destruction", func() {
				BeforeEach(func() {
					containerDestructionShouldSucceed = false
					config.ImagePluginExtraArgs = append(
						config.ImagePluginExtraArgs,
						"\"--fail-on\"",
						"destroy",
					)
				})

				It("returns the plugin's stdout in a useful error", func() {
					err := client.Destroy(container.Handle())
					Expect(err).To(MatchError(ContainSubstring("running image plugin destroy: destroy failed")))
				})
			})
		})

		Context("but the plugin fails on creation", func() {
			BeforeEach(func() {
				config.ImagePluginExtraArgs = append(
					config.ImagePluginExtraArgs,
					"\"--fail-on\"",
					"create",
				)
			})

			It("returns the plugin's stdout in a useful error", func() {
				_, err := client.Create(garden.ContainerSpec{})
				Expect(err).To(MatchError(ContainSubstring("running image plugin create: create failed")))
			})
		})

		Context("but we attempt to create a privileged container", func() {
			It("returns an informative error", func() {
				_, err := client.Create(garden.ContainerSpec{Privileged: true})
				Expect(err).To(MatchError(ContainSubstring("no privileged_image_plugin provided")))
			})
		})
	})

	Context("when only a privileged image plugin is provided", func() {
		BeforeEach(func() {
			config.PrivilegedImagePluginBin = binaries.ImagePlugin
			config.PrivilegedImagePluginExtraArgs = []string{
				"\"--rootfs-path\"",
				tmpDir,
				"\"--args-path\"",
				filepath.Join(tmpDir, "args"),
			}
		})

		Context("and a container is created", func() {
			var (
				containerSpec garden.ContainerSpec
				container     garden.Container
				handle        string
			)

			BeforeEach(func() {
				containerSpec = garden.ContainerSpec{Privileged: true}

				config.PrivilegedImagePluginExtraArgs = append(
					config.PrivilegedImagePluginExtraArgs,
					"\"--create-whoami-path\"",
					filepath.Join(tmpDir, "create-whoami"),
				)
			})

			JustBeforeEach(func() {
				var err error
				container, err = client.Create(containerSpec)
				Expect(err).NotTo(HaveOccurred())
				handle = container.Handle()
			})

			It("executes the plugin, passing the correct args", func() {
				pluginArgsBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "args"))
				Expect(err).ToNot(HaveOccurred())

				pluginArgs := strings.Split(string(pluginArgsBytes), " ")
				Expect(pluginArgs).To(Equal([]string{
					binaries.ImagePlugin,
					"--rootfs-path", tmpDir,
					"--args-path", filepath.Join(tmpDir, "args"),
					"--create-whoami-path", filepath.Join(tmpDir, "create-whoami"),
					"create",
					defaultTestRootFS,
					handle,
				}))
			})

			It("executes the plugin as root", func() {
				whoamiBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "create-whoami"))
				Expect(err).NotTo(HaveOccurred())

				Expect(string(whoamiBytes)).To(ContainSubstring("0 - 0"))
			})

			Context("when there are env vars", func() {
				BeforeEach(func() {
					image := gardener.Image{
						Config: gardener.ImageConfig{
							Env: []string{
								"MY_VAR=set",
								"MY_SECOND_VAR=also_set",
							},
						},
					}
					imageJson, err := json.Marshal(image)
					Expect(err).NotTo(HaveOccurred())

					config.PrivilegedImagePluginExtraArgs = append(
						config.PrivilegedImagePluginExtraArgs,
						"\"--image-json\"",
						string(imageJson),
					)
					gardenDefaultRootfs := defaultTestRootFS
					Expect(copyFile(filepath.Join(gardenDefaultRootfs, "bin", "env"),
						filepath.Join(tmpDir, "env"))).To(Succeed())
				})

				It("loads the image.json env variables", func() {
					buffer := gbytes.NewBuffer()
					process, err := container.Run(garden.ProcessSpec{
						Path: "/env",
						Dir:  "/",
					}, garden.ProcessIO{Stdout: buffer, Stderr: buffer})
					Expect(err).NotTo(HaveOccurred())
					exitCode, err := process.Wait()
					Expect(err).NotTo(HaveOccurred())
					Expect(exitCode).To(BeZero())

					Eventually(buffer).Should(gbytes.Say("MY_VAR=set"))
					Eventually(buffer).Should(gbytes.Say("MY_SECOND_VAR=also_set"))
				})
			})

			Context("when rootfs is not specified", func() {
				BeforeEach(func() {
					containerSpec.RootFSPath = ""
				})

				It("uses the default rootfs", func() {
					pluginArgsBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "args"))
					Expect(err).ToNot(HaveOccurred())

					Expect(string(pluginArgsBytes)).To(ContainSubstring(defaultTestRootFS))
				})
			})

			Context("when passing a tagged docker image as the RootFSPath", func() {
				BeforeEach(func() {
					containerSpec.RootFSPath = "docker:///busybox#1.26.1"
				})

				It("replaces the # with :", func() {
					pluginArgsBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "args"))
					Expect(err).ToNot(HaveOccurred())

					Expect(string(pluginArgsBytes)).To(ContainSubstring("docker:///busybox:1.26.1"))
				})
			})

			Context("when passing an Image URI", func() {
				BeforeEach(func() {
					containerSpec.Image = garden.ImageRef{URI: "/some/fake/rootfs"}
				})

				It("executes the plugin, passing the Image URI", func() {
					pluginArgsBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "args"))
					Expect(err).ToNot(HaveOccurred())

					Expect(string(pluginArgsBytes)).To(ContainSubstring("/some/fake/rootfs"))
				})
			})

			Context("when specifying a quota", func() {
				BeforeEach(func() {
					containerSpec.Limits.Disk.ByteHard = 100000
				})

				It("calls the image plugin setting the quota", func() {
					pluginArgsBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "args"))
					Expect(err).ToNot(HaveOccurred())

					Expect(string(pluginArgsBytes)).To(ContainSubstring("--disk-limit-size-bytes 100000"))
				})

				Context("when the quota is total", func() {
					BeforeEach(func() {
						containerSpec.Limits.Disk.Scope = garden.DiskLimitScopeTotal
					})

					It("calls the image plugin without the exclusive flag", func() {
						pluginArgsBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "args"))
						Expect(err).ToNot(HaveOccurred())

						Expect(string(pluginArgsBytes)).NotTo(ContainSubstring("--exclude-image-from-quota"))
					})
				})

				Context("when the quota is exclusive", func() {
					BeforeEach(func() {
						containerSpec.Limits.Disk.Scope = garden.DiskLimitScopeExclusive
					})

					It("calls the image plugin setting the exclusive flag", func() {
						pluginArgsBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "args"))
						Expect(err).ToNot(HaveOccurred())

						Expect(string(pluginArgsBytes)).To(ContainSubstring("--exclude-image-from-quota"))
					})
				})
			})

			Context("and metrics are collected on that container", func() {
				BeforeEach(func() {
					config.PrivilegedImagePluginExtraArgs = append(
						config.PrivilegedImagePluginExtraArgs,
						"\"--metrics-whoami-path\"",
						filepath.Join(tmpDir, "metrics-whoami"),
						"\"--metrics-output\"",
						`{"disk_usage": {"total_bytes_used": 1000, "exclusive_bytes_used": 2000}}`,
					)
				})

				JustBeforeEach(func() {
					_, err := container.Metrics()
					Expect(err).NotTo(HaveOccurred())
				})

				It("executes the plugin, passing the correct args", func() {
					pluginArgsBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "args"))
					Expect(err).ToNot(HaveOccurred())

					pluginArgs := strings.Split(string(pluginArgsBytes), " ")
					Expect(pluginArgs).To(Equal([]string{
						binaries.ImagePlugin,
						"--rootfs-path", tmpDir,
						"--args-path", filepath.Join(tmpDir, "args"),
						"--create-whoami-path", filepath.Join(tmpDir, "create-whoami"),
						"--metrics-whoami-path", filepath.Join(tmpDir, "metrics-whoami"),
						"--metrics-output",
						"{\"disk_usage\":",
						"{\"total_bytes_used\":",
						"1000,",
						"\"exclusive_bytes_used\":",
						"2000}}",
						"stats",
						handle,
					}))
				})

				It("executes the plugin as root", func() {
					whoamiBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "metrics-whoami"))
					Expect(err).NotTo(HaveOccurred())

					Expect(string(whoamiBytes)).To(ContainSubstring("0 - 0"))
				})

				Context("when the plugin logs to stderr", func() {
					BeforeEach(func() {
						config.PrivilegedImagePluginExtraArgs = append(
							config.PrivilegedImagePluginExtraArgs,
							"\"--metrics-log-content\"",
							"METRICS-FAKE-LOG-LINE",
						)
					})

					It("relogs the plugin's stderr to the garden logs", func() {
						Eventually(client).Should(gbytes.Say("METRICS-FAKE-LOG-LINE"))
					})
				})
			})

			Context("but the plugin fails when collecting metrics", func() {
				BeforeEach(func() {
					config.PrivilegedImagePluginExtraArgs = append(
						config.PrivilegedImagePluginExtraArgs,
						"\"--fail-on\"",
						"metrics",
					)
				})

				It("returns the plugin's stdout in a useful error", func() {
					_, err := container.Metrics()
					Expect(err).To(MatchError(ContainSubstring("running image plugin metrics: metrics failed")))
				})
			})

			Context("and that container is destroyed", func() {
				BeforeEach(func() {
					config.PrivilegedImagePluginExtraArgs = append(
						config.PrivilegedImagePluginExtraArgs,
						"\"--destroy-whoami-path\"",
						filepath.Join(tmpDir, "destroy-whoami"),
					)
				})

				JustBeforeEach(func() {
					Expect(client.Destroy(container.Handle())).Should(Succeed())
				})

				It("executes the plugin, passing the correct args", func() {
					pluginArgsBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "args"))
					Expect(err).ToNot(HaveOccurred())

					pluginArgs := strings.Split(string(pluginArgsBytes), " ")
					Expect(pluginArgs).To(Equal([]string{
						binaries.ImagePlugin,
						"--rootfs-path", tmpDir,
						"--args-path", filepath.Join(tmpDir, "args"),
						"--create-whoami-path", filepath.Join(tmpDir, "create-whoami"),
						"--destroy-whoami-path", filepath.Join(tmpDir, "destroy-whoami"),
						"delete",
						handle,
					}))
				})

				It("executes the plugin as root", func() {
					whoamiBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "destroy-whoami"))
					Expect(err).NotTo(HaveOccurred())

					Expect(string(whoamiBytes)).To(ContainSubstring("0 - 0"))
				})

				Context("when the plugin logs to stderr", func() {
					BeforeEach(func() {
						config.PrivilegedImagePluginExtraArgs = append(
							config.PrivilegedImagePluginExtraArgs,
							"\"--destroy-log-content\"",
							"DESTROY-FAKE-LOG-LINE",
						)
					})

					It("relogs the plugin's stderr to the garden logs", func() {
						Eventually(client).Should(gbytes.Say("DESTROY-FAKE-LOG-LINE"))
					})
				})
			})

			Context("but the plugin fails on destruction", func() {
				BeforeEach(func() {
					containerDestructionShouldSucceed = false
					config.PrivilegedImagePluginExtraArgs = append(
						config.PrivilegedImagePluginExtraArgs,
						"\"--fail-on\"",
						"destroy",
					)
				})

				It("returns the plugin's stdout in a useful error", func() {
					err := client.Destroy(container.Handle())
					Expect(err).To(MatchError(ContainSubstring("running image plugin destroy: destroy failed")))
				})
			})
		})

		Context("but the plugin fails on creation", func() {
			BeforeEach(func() {
				config.PrivilegedImagePluginExtraArgs = append(
					config.PrivilegedImagePluginExtraArgs,
					"\"--fail-on\"",
					"create",
				)
			})

			It("returns the plugin's stdout in a useful error", func() {
				_, err := client.Create(garden.ContainerSpec{Privileged: true})
				Expect(err).To(MatchError(ContainSubstring("running image plugin create: create failed")))
			})
		})

		Context("but we attempt to create an unprivileged container", func() {
			It("returns an informative error", func() {
				_, err := client.Create(garden.ContainerSpec{Privileged: false})
				Expect(err).To(MatchError(ContainSubstring("no image_plugin provided")))
			})
		})
	})

	Context("when both image_plugin and privileged_image_plugin are provided", func() {
		BeforeEach(func() {
			// make a a copy of the fake image plugin so we can check location of file called
			privilegedImagePluginPath := fmt.Sprintf("%s-priv", binaries.ImagePlugin)
			Expect(copyFile(binaries.ImagePlugin, privilegedImagePluginPath)).To(Succeed())

			config.ImagePluginBin = binaries.ImagePlugin
			config.PrivilegedImagePluginBin = privilegedImagePluginPath
			config.ImagePluginExtraArgs = append(
				config.ImagePluginExtraArgs,
				"\"--rootfs-path\"",
				tmpDir,
				"\"--create-bin-location-path\"",
				filepath.Join(tmpDir, "create-bin-location"),
				"\"--destroy-bin-location-path\"",
				filepath.Join(tmpDir, "destroy-bin-location"),
				"\"--metrics-bin-location-path\"",
				filepath.Join(tmpDir, "metrics-bin-location"),
			)
			config.PrivilegedImagePluginExtraArgs = append(
				config.PrivilegedImagePluginExtraArgs,
				"\"--rootfs-path\"",
				tmpDir,
				"\"--create-bin-location-path\"",
				filepath.Join(tmpDir, "create-bin-location"),
				"\"--destroy-bin-location-path\"",
				filepath.Join(tmpDir, "destroy-bin-location"),
				"\"--metrics-bin-location-path\"",
				filepath.Join(tmpDir, "metrics-bin-location"),
			)
		})

		Context("when an unprivileged container is created", func() {
			var container garden.Container

			JustBeforeEach(func() {
				var err error
				container, err = client.Create(garden.ContainerSpec{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("calls only the unprivileged plugin", func() {
				pluginLocationBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "create-bin-location"))
				Expect(err).ToNot(HaveOccurred())

				Expect(string(pluginLocationBytes)).To(Equal(binaries.ImagePlugin))
			})

			Context("and metrics are collected on that container", func() {
				JustBeforeEach(func() {
					_, err := container.Metrics()
					Expect(err).NotTo(HaveOccurred())
				})

				It("calls only the unprivileged plugin", func() {
					pluginLocationBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "metrics-bin-location"))
					Expect(err).ToNot(HaveOccurred())

					Expect(string(pluginLocationBytes)).To(Equal(binaries.ImagePlugin))
				})
			})

			Context("and that container is destroyed", func() {
				JustBeforeEach(func() {
					Expect(client.Destroy(container.Handle())).To(Succeed())
				})

				It("calls the unprivileged plugin", func() {
					pluginLocationBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "destroy-bin-location"))
					Expect(err).ToNot(HaveOccurred())

					Expect(string(pluginLocationBytes)).To(ContainSubstring(binaries.ImagePlugin))
				})
			})
		})

		Context("when a privileged container is created", func() {
			var container garden.Container

			JustBeforeEach(func() {
				var err error
				container, err = client.Create(garden.ContainerSpec{Privileged: true})
				Expect(err).NotTo(HaveOccurred())
			})

			It("calls only the privileged plugin", func() {
				pluginLocationBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "create-bin-location"))
				Expect(err).ToNot(HaveOccurred())

				Expect(string(pluginLocationBytes)).To(Equal(fmt.Sprintf("%s-priv", binaries.ImagePlugin)))
			})

			Context("and metrics are collected on that container", func() {
				JustBeforeEach(func() {
					_, err := container.Metrics()
					Expect(err).NotTo(HaveOccurred())
				})

				It("calls only the privileged plugin", func() {
					pluginLocationBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "metrics-bin-location"))
					Expect(err).ToNot(HaveOccurred())

					Expect(string(pluginLocationBytes)).To(Equal(fmt.Sprintf("%s-priv", binaries.ImagePlugin)))
				})
			})

			Context("and that container is destroyed", func() {
				JustBeforeEach(func() {
					Expect(client.Destroy(container.Handle())).To(Succeed())
				})

				It("calls the privileged plugin", func() {
					pluginLocationBytes, err := ioutil.ReadFile(filepath.Join(tmpDir, "destroy-bin-location"))
					Expect(err).ToNot(HaveOccurred())

					Expect(string(pluginLocationBytes)).To(ContainSubstring(fmt.Sprintf("%s-priv", binaries.ImagePlugin)))
				})
			})
		})
	})

	Context("when images are located in a private registry", func() {
		var (
			imageSpec garden.ImageRef
		)

		BeforeEach(func() {
			config.LogLevel = "debug"
			config.ImagePluginBin = binaries.ImagePlugin
			config.ImagePluginExtraArgs = append(
				config.ImagePluginExtraArgs,
				"\"--rootfs-path\"",
				tmpDir,
				"\"--args-path\"",
				filepath.Join(tmpDir, "args"),
			)

			imageSpec = garden.ImageRef{
				URI:      "",
				Username: "imagepluginuser",
				Password: "secretpassword",
			}
		})

		It("calls the image plugin with username and password", func() {
			_, err := client.Create(garden.ContainerSpec{
				Image: imageSpec,
			})
			Expect(err).NotTo(HaveOccurred())

			args, err := ioutil.ReadFile(filepath.Join(tmpDir, "args"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(args)).To(ContainSubstring("imagepluginuser"))
			Expect(string(args)).To(ContainSubstring("secretpassword"))
		})

		It("does not log username and password", func() {
			_, err := client.Create(garden.ContainerSpec{
				Image: imageSpec,
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(client, "1s").ShouldNot(gbytes.Say("imagepluginuser"))
			Eventually(client, "1s").ShouldNot(gbytes.Say("secretpassword"))
		})

	})
})

func copyFile(srcPath, dstPath string) error {
	dirPath := filepath.Dir(dstPath)
	if err := os.MkdirAll(dirPath, 0777); err != nil {
		return err
	}

	reader, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	writer, err := os.Create(dstPath)
	if err != nil {
		reader.Close()
		return err
	}

	if _, err := io.Copy(writer, reader); err != nil {
		writer.Close()
		reader.Close()
		return err
	}

	writer.Close()
	reader.Close()

	return os.Chmod(writer.Name(), 0777)
}

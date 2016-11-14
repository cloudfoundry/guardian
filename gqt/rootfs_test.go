package gqt_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	"code.cloudfoundry.org/guardian/sysinfo"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/types"
)

var dockerRegistryV2RootFSPath = os.Getenv("GARDEN_DOCKER_REGISTRY_V2_TEST_ROOTFS")

var _ = Describe("Rootfs container create parameter", func() {
	var container garden.Container
	var args []string
	var client *runner.RunningGarden
	var supplyDefaultRootfs bool

	BeforeEach(func() {
		container = nil
		args = []string{}
	})

	JustBeforeEach(func() {
		if supplyDefaultRootfs {
			client = startGarden(args...)
		} else {
			client = startGardenWithoutDefaultRootfs(args...)
		}
	})

	AfterEach(func() {
		if container != nil {
			Expect(client.Destroy(container.Handle())).To(Succeed())
		}
	})

	Context("without a default rootfs", func() {
		BeforeEach(func() {
			supplyDefaultRootfs = false
		})

		It("fails if a rootfs is not supplied in container spec", func() {
			var err error

			container, err = client.Create(garden.ContainerSpec{RootFSPath: ""})
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("RootFSPath: is a required parameter, since no default rootfs was provided to the server.")))
		})

		It("creates successfully if a rootfs is supplied in container spec", func() {
			var err error

			container, err = client.Create(garden.ContainerSpec{RootFSPath: os.Getenv("GARDEN_TEST_ROOTFS")})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("with a default rootfs", func() {
		BeforeEach(func() {
			args = append(args, "--default-rootfs", os.Getenv("GARDEN_TEST_ROOTFS"))
		})

		It("the container is created successfully", func() {
			var err error

			container, err = client.Create(garden.ContainerSpec{RootFSPath: ""})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("with an empty rootfs", func() {
		It("creates the container successfully", func() {
			rootfs, err := ioutil.TempDir("", "emptyrootfs")
			Expect(err).NotTo(HaveOccurred())

			_, err = client.Create(garden.ContainerSpec{RootFSPath: rootfs})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("with a docker rootfs URI", func() {
		Context("not containing a host", func() {
			It("succesfully creates the container", func() {
				var err error

				container, err = client.Create(garden.ContainerSpec{RootFSPath: "docker:///cfgarden/garden-busybox"})
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when image does not exist", func() {
				It("should not leak the depot directory", func() {
					_, err := client.Create(
						garden.ContainerSpec{
							RootFSPath: "docker:///cloudfoundry/doesnotexist",
						},
					)
					Expect(err).To(HaveOccurred())

					entries, err := ioutil.ReadDir(client.DepotDir)
					Expect(err).ToNot(HaveOccurred())
					Expect(entries).To(HaveLen(0))
				})
			})

			Context("when the -registry flag targets a non-existing registry", func() {
				BeforeEach(func() {
					args = []string{"--docker-registry", "registry-12.banana-docker.io"}
				})

				It("should fail to create a container", func() {
					var err error

					container, err = client.Create(garden.ContainerSpec{RootFSPath: "docker:///busybox"})
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("containing a host", func() {
			Context("which is valid", func() {
				It("creates the container successfully", func() {
					var err error

					container, err = client.Create(garden.ContainerSpec{RootFSPath: "docker://registry-1.docker.io/cfgarden/garden-busybox"})
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("which is invalid", func() {
				It("the container is not created successfully", func() {
					var err error
					container, err = client.Create(garden.ContainerSpec{RootFSPath: "docker://xindex.docker.io/busybox"})
					Expect(err).To(HaveOccurred())
				})
			})

			Context("which is insecure", func() {
				var (
					dockerRegistry     garden.Container
					dockerRegistryIP   string
					dockerRegistryPort string
				)

				BeforeEach(func() {
					dockerRegistryIP = "192.168.12.34"
					dockerRegistryPort = "5000"
				})

				JustBeforeEach(func() {
					if dockerRegistryV2RootFSPath == "" {
						Skip("GARDEN_DOCKER_REGISTRY_V2_TEST_ROOTFS undefined")
					}

					dockerRegistry = startV2DockerRegistry(client, dockerRegistryIP, dockerRegistryPort)
				})

				AfterEach(func() {
					if dockerRegistry != nil {
						Expect(client.Destroy(dockerRegistry.Handle())).To(Succeed())
					}
				})

				Context("when the host is listed in --insecure-docker-registry", func() {
					BeforeEach(func() {
						args = []string{"--allow-host-access"}
					})

					Context("when the registry is NOT using TLS", func() {
						BeforeEach(func() {
							args = append(
								args,
								"--insecure-docker-registry",
								fmt.Sprintf("%s:%s", dockerRegistryIP, dockerRegistryPort),
							)
						})

						It("creates the container successfully ", func() {
							_, err := client.Create(garden.ContainerSpec{
								RootFSPath: fmt.Sprintf("docker://%s:%s/busybox", dockerRegistryIP,
									dockerRegistryPort),
								// This container does not need to be privileged. However,
								// Garden-Runc cannot create non-privileged containers that use
								// docker:///busybox. It turns out that runC fails to create
								// `/proc` inside the container.
								Privileged: true,
							})
							Expect(err).ToNot(HaveOccurred())
						})
					})

					Context("when the registry is in a CIDR", func() {
						BeforeEach(func() {
							args = append(
								args,
								"--insecure-docker-registry",
								fmt.Sprintf("%s/24", dockerRegistryIP),
							)
						})

						It("creates the container successfully ", func() {
							_, err := client.Create(garden.ContainerSpec{
								RootFSPath: fmt.Sprintf("docker://%s:%s/busybox", dockerRegistryIP, dockerRegistryPort),
								// This container does not need to be privileged. However,
								// Guardian cannot create non-privileged containers that use
								// docker:///busybox. It turns out that runC fails to create
								// `/proc` inside the container.
								Privileged: true,
							})
							Expect(err).ToNot(HaveOccurred())
						})
					})

					Context("when the registry is using TLS", func() {
						var server *httptest.Server
						var serverURL *url.URL

						BeforeEach(func() {
							proxyTo, err := url.Parse(fmt.Sprintf("http://%s:%s", dockerRegistryIP,
								dockerRegistryPort))
							Expect(err).NotTo(HaveOccurred())

							server = httptest.NewTLSServer(httputil.NewSingleHostReverseProxy(proxyTo))
							serverURL, err = url.Parse(server.URL)
							Expect(err).NotTo(HaveOccurred())

							args = append(
								args,
								"--insecure-docker-registry",
								serverURL.Host,
							)
						})

						AfterEach(func() {
							server.Close()
						})

						It("creates the container successfully", func() {
							_, err := client.Create(garden.ContainerSpec{
								RootFSPath: fmt.Sprintf("docker://%s/busybox", serverURL.Host),
								// This container does not need to be privileged. However,
								// Guardian cannot create non-privileged containers that use
								// docker:///busybox. It turns out that runC fails to create
								// `/proc` inside the container.
								Privileged: true,
							})
							Expect(err).ToNot(HaveOccurred())
						})

						Context("and its specified as --registry", func() {
							BeforeEach(func() {
								args = append(args, "--docker-registry", serverURL.Host)
							})

							It("still works when the host is specified", func() {
								_, err := client.Create(garden.ContainerSpec{
									RootFSPath: fmt.Sprintf("docker://%s/busybox", serverURL.Host),
									// This container does not need to be privileged. However,
									// Guardian cannot create non-privileged containers that use
									// docker:///busybox. It turns out that runC fails to create
									// `/proc` inside the container.
									Privileged: true,
								})
								Expect(err).ToNot(HaveOccurred())
							})

							It("still works using the default host", func() {
								_, err := client.Create(garden.ContainerSpec{
									RootFSPath: "docker:///busybox",
									// This container does not need to be privileged. However,
									// Guardian cannot create non-privileged containers that use
									// docker:///busybox. It turns out that runC fails to create
									// `/proc` inside the container.
									Privileged: true,
								})
								Expect(err).ToNot(HaveOccurred())
							})
						})
					})
				})

				Context("when the host is NOT listed in -insecureDockerRegistry", func() {
					It("fails", func() {
						_, err := client.Create(garden.ContainerSpec{
							RootFSPath: fmt.Sprintf("docker://%s:%s/busybox", dockerRegistryIP,
								dockerRegistryPort),
						})

						Expect(err).To(HaveOccurred())
					})
				})
			})
		})
	})

	Context("when an image plugin path is provided at startup", func() {
		var (
			imageID    string
			imagePath  string
			storePath  string
			privileged bool
		)

		BeforeEach(func() {
			args = append(args, "--image-plugin", testImagePluginBin)
		})

		AfterEach(func() {
			Expect(os.RemoveAll(imagePath)).To(Succeed())
		})

		Context("and the container is privileged", func() {
			BeforeEach(func() {
				storePath = "/tmp/store-path" // we can't use ioutil.TempDir as the fake image plugin needs to know the directory
				privileged = true
			})

			Context("and a non-quotaed container is created and destroyed", func() {
				JustBeforeEach(func() {
					imageID = fmt.Sprintf("non-quotaed-container-%d", GinkgoParallelNode())
					imagePath = filepath.Join(storePath, imageID)

					c, err := client.Create(garden.ContainerSpec{
						RootFSPath: "docker:///cfgarden/empty#v0.1.0",
						Handle:     imageID,
						Privileged: privileged,
					})
					Expect(err).ToNot(HaveOccurred())
					client.Destroy(c.Handle())
				})

				It("executes the plugin as the host root user", func() {
					whoamiOutput, err := ioutil.ReadFile(filepath.Join(imagePath, "create-whoami"))
					Expect(err).ToNot(HaveOccurred())
					Expect(string(whoamiOutput)).To(ContainSubstring("0 - 0"))
				})

				It("does not pass any mappings to the plugin", func() {
					args, err := ioutil.ReadFile(filepath.Join(imagePath, "create-args"))
					Expect(err).ToNot(HaveOccurred())
					Expect(string(args)).NotTo(ContainSubstring("--uid-mapping"))
					Expect(string(args)).NotTo(ContainSubstring("--gid-mapping"))
				})
			})
		})

		Context("when the container is unprivileged", func() {
			BeforeEach(func() {
				storePath = "/tmp/unpriv-store-path" // we can't use ioutil.TempDir as the fake image plugin needs to know the directory
				privileged = false
			})

			Context("and a non-quotaed container is created and destroyed", func() {
				var container garden.Container

				JustBeforeEach(func() {
					imageID = fmt.Sprintf("non-quotaed-container-%d", GinkgoParallelNode())
					imagePath = filepath.Join(storePath, imageID)

					var err error
					container, err = client.Create(garden.ContainerSpec{
						RootFSPath: "docker:///cfgarden/empty#v0.1.0",
						Handle:     imageID,
						Privileged: privileged,
					})
					Expect(err).ToNot(HaveOccurred())
				})

				AfterEach(func() {
					client.Destroy(container.Handle())
				})

				It("executes plugin create with the correct args", func() {
					maxId := uint32(sysinfo.Min(sysinfo.MustGetMaxValidUID(), sysinfo.MustGetMaxValidGID()))
					userArgsStr, err := ioutil.ReadFile(filepath.Join(imagePath, "create-args"))
					Expect(err).ToNot(HaveOccurred())

					userArgs := strings.Split(string(userArgsStr), " ")
					Expect(userArgs).To(Equal([]string{
						testImagePluginBin,
						"create",
						"--uid-mapping",
						fmt.Sprintf("0:%d:1", maxId),
						"--gid-mapping",
						fmt.Sprintf("0:%d:1", maxId),
						"--uid-mapping",
						fmt.Sprintf("1:1:%d", maxId-1),
						"--gid-mapping",
						fmt.Sprintf("1:1:%d", maxId-1),
						"docker:///cfgarden/empty#v0.1.0",
						fmt.Sprintf("non-quotaed-container-%d", GinkgoParallelNode()),
					}))
				})

				It("executes plugin create as the container user", func() {
					maxId := uint32(sysinfo.Min(sysinfo.MustGetMaxValidUID(), sysinfo.MustGetMaxValidGID()))
					whoamiOutput, err := ioutil.ReadFile(filepath.Join(imagePath, "create-whoami"))
					Expect(err).ToNot(HaveOccurred())
					Expect(string(whoamiOutput)).To(ContainSubstring(fmt.Sprintf("%d - %d", maxId, maxId)))
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

					Eventually(buffer).Should(gbytes.Say("BLA=BLE"))
					Eventually(buffer).Should(gbytes.Say("HELLO=world"))
				})

				It("executes plugin delete with the correct args", func() {
					Expect(client.Destroy(container.Handle())).To(Succeed())
					userArgsStr, err := ioutil.ReadFile(filepath.Join(imagePath, "delete-args"))
					Expect(err).ToNot(HaveOccurred())

					userArgs := strings.Split(string(userArgsStr), " ")
					Expect(userArgs).To(Equal([]string{
						testImagePluginBin,
						"delete",
						filepath.Join(storePath, imageID),
					}))
				})

				It("executes plugin delete as the host root user", func() {
					Expect(client.Destroy(container.Handle())).To(Succeed())
					whoamiOutput, err := ioutil.ReadFile(filepath.Join(imagePath, "delete-whoami"))
					Expect(err).ToNot(HaveOccurred())
					Expect(string(whoamiOutput)).To(ContainSubstring("0 - 0"))
				})
			})

			Context("and a quotaed container is created", func() {
				JustBeforeEach(func() {
					imageID = fmt.Sprintf("quotaed-container-%d", GinkgoParallelNode())
					imagePath = filepath.Join(storePath, imageID)
					_, err := client.Create(garden.ContainerSpec{
						RootFSPath: "docker:///cfgarden/empty#v0.1.0",
						Handle:     imageID,
						Privileged: false,
						Limits: garden.Limits{
							Disk: garden.DiskLimits{
								ByteHard: 1 * 1024 * 1024,
							},
						},
					})
					Expect(err).ToNot(HaveOccurred())
				})

				It("passes the disk limit to the image plugin as an argument", func() {
					args, err := ioutil.ReadFile(filepath.Join(imagePath, fmt.Sprintf("create-args")))
					Expect(err).ToNot(HaveOccurred())
					Expect(string(args)).To(ContainSubstring("--disk-limit-size-bytes 1048576"))
				})
			})

			Context("when the image plugin fails during creation", func() {
				It("provides a sensible error message", func() {
					_, err := client.Create(garden.ContainerSpec{
						RootFSPath: "docker:///cfgarden/empty#v0.1.0",
						Handle:     "make-it-fail",
						Privileged: false,
					})
					Expect(err).To(MatchError(ContainSubstring("external image manager create failed")))
				})
			})

			Context("when the image plugin fails during destruction", func() {
				It("provides a sensible error message", func() {
					_, err := client.Create(garden.ContainerSpec{
						RootFSPath: "docker:///cfgarden/empty#v0.1.0",
						Handle:     "make-it-fail-on-destruction",
						Privileged: false,
					})
					Expect(err).ToNot(HaveOccurred())

					err = client.Destroy("make-it-fail-on-destruction")
					Expect(err).To(MatchError(ContainSubstring("external image manager destroy failed")))
				})
			})

			Context("and a container is created without specifying a RootFSPath", func() {
				var fakeRootFsDir string

				BeforeEach(func() {
					var err error
					fakeRootFsDir, err = ioutil.TempDir("", "")
					Expect(err).NotTo(HaveOccurred())
					args = append(args, "--default-rootfs", fakeRootFsDir)
				})

				JustBeforeEach(func() {
					imageID = fmt.Sprintf("default-rootfs-container-%d", GinkgoParallelNode())
					imagePath = filepath.Join(storePath, imageID)

					_, err := client.Create(garden.ContainerSpec{
						Handle: imageID,
					})
					Expect(err).ToNot(HaveOccurred())
				})

				AfterEach(func() {
					Expect(os.RemoveAll(filepath.Join(storePath, imageID))).To(Succeed())
					Expect(os.RemoveAll(fakeRootFsDir)).To(Succeed())
				})

				It("should pass the value of the --default-rootfs startup flag to the image plugin", func() {
					args, err := ioutil.ReadFile(filepath.Join(imagePath, "create-args"))
					Expect(err).ToNot(HaveOccurred())
					Expect(string(args)).To(ContainSubstring(fakeRootFsDir))
				})
			})
		})
	})

	Context("when the modified timestamp of the rootfs top-level directory changes", func() {
		var (
			rootfspath          string
			privilegedContainer bool
			container2          garden.Container
		)

		JustBeforeEach(func() {
			var err error
			rootfspath = createSmallRootfs()

			container, err = client.Create(garden.ContainerSpec{
				RootFSPath: rootfspath,
				Privileged: privilegedContainer,
			})
			Expect(err).NotTo(HaveOccurred())

			// ls is convenient, but any file modification is sufficient
			ls := filepath.Join(rootfspath, "bin", "ls")
			Expect(exec.Command("cp", ls, rootfspath).Run()).To(Succeed())

			container2, err = client.Create(garden.ContainerSpec{
				RootFSPath: rootfspath,
				Privileged: privilegedContainer,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			if container2 != nil {
				Expect(client.Destroy(container2.Handle())).To(Succeed())
			}
		})

		Context("with a non-privileged container", func() {
			BeforeEach(func() {
				privilegedContainer = false
			})

			It("should use the updated rootfs when creating a new container", func() {
				process, err := container2.Run(garden.ProcessSpec{
					Path: "/ls",
					User: "root",
				}, garden.ProcessIO{Stdout: GinkgoWriter, Stderr: GinkgoWriter})
				Expect(err).NotTo(HaveOccurred())

				exitStatus, err := process.Wait()
				Expect(err).NotTo(HaveOccurred())
				Expect(exitStatus).To(Equal(0))
			})
		})
	})

})

func startV2DockerRegistry(client garden.Client, dockerRegistryIP string, dockerRegistryPort string) garden.Container {
	dockerRegistry, err := client.Create(
		garden.ContainerSpec{
			RootFSPath: dockerRegistryV2RootFSPath,
			Network:    dockerRegistryIP,
		},
	)
	Expect(err).ToNot(HaveOccurred())

	_, err = dockerRegistry.Run(garden.ProcessSpec{
		User: "root",
		Env: []string{
			"REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY=/opt/docker-registry",
		},
		Path: "/go/bin/registry",
		Args: []string{"/go/src/github.com/docker/distribution/cmd/registry/config.yml"},
	}, garden.ProcessIO{Stdout: GinkgoWriter, Stderr: GinkgoWriter})
	Expect(err).ToNot(HaveOccurred())

	Eventually(
		fmt.Sprintf("http://%s:%s/v2/", dockerRegistryIP, dockerRegistryPort),
		"60s",
	).Should(RespondToGETWith(200))

	return dockerRegistry
}

type statusMatcher struct {
	expectedStatus int

	httpError    error
	actualStatus int
}

func RespondToGETWith(expected int) types.GomegaMatcher {
	return &statusMatcher{expected, nil, 200}
}

func (m *statusMatcher) Match(actual interface{}) (success bool, err error) {
	response, err := http.Get(fmt.Sprintf("%s", actual))
	if err != nil {
		m.httpError = err
		return false, nil
	}

	m.httpError = nil
	m.actualStatus = response.StatusCode
	return response.StatusCode == m.expectedStatus, nil
}

func (m *statusMatcher) FailureMessage(actual interface{}) string {
	if m.httpError != nil {
		return fmt.Sprintf("Expected http request to have status %d but got error: %s", m.expectedStatus, m.httpError.Error())
	}

	return fmt.Sprintf("Expected http status code to be %d but was %d", m.expectedStatus, m.actualStatus)
}

func (m *statusMatcher) NegatedFailureMessage(actual interface{}) string {
	if m.httpError != nil {
		return fmt.Sprintf("Expected http request to have status %d, but got error: %s", m.expectedStatus, m.httpError.Error())
	}

	return fmt.Sprintf("Expected http status code not to be %d", m.expectedStatus)
}

func createSmallRootfs() string {
	rootfs := os.Getenv("GARDEN_PREEXISTING_USERS_TEST_ROOTFS")
	if rootfs == "" {
		Skip("pre-existing users rootfs not found")
	}

	rootfspath, err := ioutil.TempDir("", "rootfs-cache-invalidation")
	Expect(err).NotTo(HaveOccurred())
	cmd := exec.Command("cp", "-rf", rootfs, rootfspath)
	cmd.Stdout = GinkgoWriter
	cmd.Stderr = GinkgoWriter
	Expect(cmd.Run()).To(Succeed())
	return filepath.Join(rootfspath, filepath.Base(rootfs))
}

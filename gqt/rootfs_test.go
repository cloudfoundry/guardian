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

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

var dockerRegistryV2RootFSPath = os.Getenv("GARDEN_DOCKER_REGISTRY_V2_TEST_ROOTFS")

var _ = Describe("Rootfs container create parameter", func() {
	var client *runner.RunningGarden

	JustBeforeEach(func() {
		client = runner.Start(config)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	Context("with an Image URI provided", func() {
		It("creates a container using that URI as the rootfs", func() {
			_, err := client.Create(garden.ContainerSpec{Image: garden.ImageRef{URI: "docker:///cfgarden/garden-busybox"}})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when Image URI and RootFSPath are both specified", func() {
		It("returns an informative error message", func() {
			_, err := client.Create(garden.ContainerSpec{Image: garden.ImageRef{URI: "docker:///cfgarden/garden-busybox"}, RootFSPath: "docker:///cfgarden/garden-busybox"})
			Expect(err).To(MatchError(ContainSubstring("Cannot provide both Image.URI and RootFSPath")))
		})
	})

	Context("without a default rootfs", func() {
		BeforeEach(func() {
			config.DefaultRootFS = ""
		})

		It("fails if a rootfs is not supplied in container spec", func() {
			_, err := client.Create(garden.ContainerSpec{RootFSPath: ""})
			Expect(err).To(MatchError(ContainSubstring("RootFSPath: is a required parameter, since no default rootfs was provided to the server.")))
		})

		It("creates successfully if a rootfs is supplied in container spec", func() {
			_, err := client.Create(garden.ContainerSpec{RootFSPath: defaultTestRootFS})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("with a default rootfs", func() {
		BeforeEach(func() {
			config.DefaultRootFS = defaultTestRootFS
		})

		It("the container is created successfully", func() {
			_, err := client.Create(garden.ContainerSpec{RootFSPath: "", Image: garden.ImageRef{URI: ""}})
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
				_, err := client.Create(garden.ContainerSpec{RootFSPath: "docker:///cfgarden/garden-busybox"})
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
					config.DockerRegistry = "registry-12.banana-docker.io"
				})

				It("should fail to create a container", func() {
					_, err := client.Create(garden.ContainerSpec{RootFSPath: "docker:///busybox"})
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("containing a host", func() {
			Context("which is valid", func() {
				It("creates the container successfully", func() {
					_, err := client.Create(garden.ContainerSpec{RootFSPath: "docker://registry-1.docker.io/cfgarden/garden-busybox"})
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("which is invalid", func() {
				It("the container is not created successfully", func() {
					_, err := client.Create(garden.ContainerSpec{RootFSPath: "docker://xindex.docker.io/busybox"})
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
						config.AllowHostAccess = boolptr(true)
					})

					Context("when the registry is NOT using TLS", func() {
						BeforeEach(func() {
							config.InsecureDockerRegistry = fmt.Sprintf("%s:%s", dockerRegistryIP, dockerRegistryPort)
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
							config.InsecureDockerRegistry = fmt.Sprintf("%s/24", dockerRegistryIP)
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

							config.InsecureDockerRegistry = serverURL.Host
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
								config.DockerRegistry = serverURL.Host
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

	Context("when the modified timestamp of the rootfs top-level directory changes", func() {
		var container2 garden.Container

		JustBeforeEach(func() {
			rootfspath := createSmallRootfs()

			_, err := client.Create(garden.ContainerSpec{
				RootFSPath: rootfspath,
			})
			Expect(err).NotTo(HaveOccurred())

			// ls is convenient, but any file modification is sufficient
			ls := filepath.Join(rootfspath, "bin", "ls")
			Expect(exec.Command("cp", ls, rootfspath).Run()).To(Succeed())

			container2, err = client.Create(garden.ContainerSpec{
				RootFSPath: rootfspath,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should use the updated rootfs when running a process", func() {
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

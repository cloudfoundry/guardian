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
	yaml "gopkg.in/yaml.v2"

	grootconf "code.cloudfoundry.org/grootfs/commands/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

var dockerRegistryV2RootFSPath = os.Getenv("GARDEN_DOCKER_REGISTRY_V2_TEST_ROOTFS")

var _ = Describe("Rootfs container create parameter", func() {
	var (
		client          *runner.RunningGarden
		grootfsConfPath string
	)

	JustBeforeEach(func() {
		grootfsConfPath = writeGrootConfig(config.InsecureDockerRegistry)
		config.ImagePluginExtraArgs = append(config.ImagePluginExtraArgs, `"--config"`, grootfsConfPath)

		client = runner.Start(config)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
		Expect(os.RemoveAll(grootfsConfPath)).To(Succeed())
	})

	Context("without an image plugin", func() {
		var rootfsPath string
		BeforeEach(func() {
			config = resetImagePluginConfig()
			rootfsPath = createRootfs(func(string) {}, 0755)
		})

		AfterEach(func() {
			Expect(os.RemoveAll(rootfsPath)).To(Succeed())
		})

		It("does not error when creating a container from a raw rootfs", func() {
			_, err := client.Create(garden.ContainerSpec{
				Image: garden.ImageRef{URI: fmt.Sprintf("raw://%s", rootfsPath)},
			})
			Expect(err).NotTo(HaveOccurred())
		})
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
			Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
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
			rootfs := filepath.Join(tempDir("", "emptyrootfs"), "empty.tar")
			runCommand(exec.Command("tar", "cvf", rootfs, "-T", "/dev/null"))

			_, err := client.Create(garden.ContainerSpec{RootFSPath: rootfs})
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
					Expect(err).To(MatchError(ContainSubstring("no such host")))
				})
			})

			Context("which is insecure", func() {
				var (
					dockerRegistry     garden.Container
					dockerRegistryIP   string
					dockerRegistryPort string
				)

				BeforeEach(func() {
					dockerRegistryIP = fmt.Sprintf("192.168.12.%d", 30+GinkgoParallelNode()*4)
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
							})
							Expect(err).ToNot(HaveOccurred())
						})
					})

					Context("when the registry is in a CIDR", func() {
						BeforeEach(func() {
							config.InsecureDockerRegistry = fmt.Sprintf("%s/24", dockerRegistryIP)
						})

						It("creates the container successfully ", func() {
							Skip("Does not work with groot")

							_, err := client.Create(garden.ContainerSpec{
								RootFSPath: fmt.Sprintf("docker://%s:%s/busybox", dockerRegistryIP, dockerRegistryPort),
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
								})
								Expect(err).ToNot(HaveOccurred())
							})

							It("still works using the default host", func() {
								_, err := client.Create(garden.ContainerSpec{
									RootFSPath: "docker:///busybox",
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
		Env: []string{
			"REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY=/opt/docker-registry",
		},
		Path: "/entrypoint.sh",
		Args: []string{"/etc/docker/registry/config.yml"},
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

func writeGrootConfig(insecureRegistry string) string {
	grootConf := grootconf.Config{
		Create: grootconf.Create{
			InsecureRegistries: []string{insecureRegistry},
		},
	}
	confYml, err := yaml.Marshal(grootConf)
	Expect(err).NotTo(HaveOccurred())

	confPath := tempFile(config.TmpDir, "groot_config")
	defer confPath.Close()

	Expect(ioutil.WriteFile(confPath.Name(), confYml, 0600)).To(Succeed())

	return confPath.Name()
}

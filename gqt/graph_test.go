package gqt_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("graph flags", func() {
	var (
		client               *runner.RunningGarden
		layersPath           string
		diffPath             string
		mntPath              string
		nonDefaultRootfsPath string
		args                 []string
		persistentImages     []string
	)

	numLayersInGraph := func() int {
		layerFiles, err := ioutil.ReadDir(layersPath)
		Expect(err).ToNot(HaveOccurred())
		diffFiles, err := ioutil.ReadDir(diffPath)
		Expect(err).ToNot(HaveOccurred())
		mntFiles, err := ioutil.ReadDir(mntPath)
		Expect(err).ToNot(HaveOccurred())

		numLayerFiles := len(layerFiles)
		Expect(numLayerFiles).To(Equal(len(diffFiles)))
		Expect(numLayerFiles).To(Equal(len(mntFiles)))
		return numLayerFiles
	}

	expectLayerCountAfterGraphCleanupToBe := func(layerCount int) {
		nonPersistantRootfsContainer, err := client.Create(garden.ContainerSpec{
			RootFSPath: nonDefaultRootfsPath,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(client.Destroy(nonPersistantRootfsContainer.Handle())).To(Succeed())
		Expect(numLayersInGraph()).To(Equal(layerCount + 2)) // +2 for the layers created for the nondefaultrootfs container
	}

	BeforeEach(func() {
		var err error
		nonDefaultRootfsPath, err = ioutil.TempDir("", "tmpRootfs")
		Expect(err).ToNot(HaveOccurred())
		// temporary workaround as runc expects a /tmp dir to exist in the container rootfs
		err = os.Mkdir(filepath.Join(nonDefaultRootfsPath, "tmp"), 0700)
		Expect(err).ToNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		for _, image := range persistentImages {
			args = append(args, "--persistent-image", image)
		}
		client = startGarden(args...)

		layersPath = path.Join(client.GraphPath, "aufs", "layers")
		diffPath = path.Join(client.GraphPath, "aufs", "diff")
		mntPath = path.Join(client.GraphPath, "aufs", "mnt")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(nonDefaultRootfsPath)).To(Succeed())
		Expect(client.DestroyAndStop()).To(Succeed())
		client.Cleanup()
	})

	Describe("--graph-cleanup-threshold-in-megabytes", func() {
		JustBeforeEach(func() {
			container, err := client.Create(garden.ContainerSpec{
				RootFSPath: "docker:///busybox",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(client.Destroy(container.Handle())).To(Succeed())
		})

		Context("when the graph cleanup threshold is set to -1", func() {
			BeforeEach(func() {
				args = []string{"--graph-cleanup-threshold-in-megabytes=-1"}
			})

			It("does NOT clean up the graph directory on create", func() {
				initialNumberOfLayers := numLayersInGraph()
				anotherContainer, err := client.Create(garden.ContainerSpec{})
				Expect(err).ToNot(HaveOccurred())

				Expect(numLayersInGraph()).To(BeNumerically(">", initialNumberOfLayers), "after creation, should NOT have deleted anything")
				Expect(client.Destroy(anotherContainer.Handle())).To(Succeed())
			})
		})

		Context("when the graph cleanup threshold is exceeded", func() {
			BeforeEach(func() {
				args = []string{"--graph-cleanup-threshold-in-megabytes", "0"}
			})

			Context("when there are other rootfs layers in the graph dir", func() {
				BeforeEach(func() {
					args = append(args, "--persistent-image", "docker:///busybox")
				})

				It("cleans up the graph directory on container creation (and not on destruction)", func() {
					restartGarden(client, "--graph-cleanup-threshold-in-megabytes=1") // restart with persistent image list empty
					Expect(numLayersInGraph()).To(BeNumerically(">", 0))

					anotherContainer, err := client.Create(garden.ContainerSpec{})
					Expect(err).ToNot(HaveOccurred())

					Expect(numLayersInGraph()).To(Equal(3), "after creation, should have deleted everything other than the default rootfs, uid translation layer and container layer")
					Expect(client.Destroy(anotherContainer.Handle())).To(Succeed())
					Expect(numLayersInGraph()).To(Equal(2), "should not garbage collect parent layers on destroy")
				})
			})
		})

		Context("when the graph cleanup threshold is not exceeded", func() {
			BeforeEach(func() {
				args = []string{"--graph-cleanup-threshold-in-megabytes", "1024"}
			})

			It("does not cleanup", func() {
				// threshold is not yet exceeded
				Expect(numLayersInGraph()).To(Equal(3))

				anotherContainer, err := client.Create(garden.ContainerSpec{})
				Expect(err).ToNot(HaveOccurred())

				Expect(numLayersInGraph()).To(Equal(6))
				Expect(client.Destroy(anotherContainer.Handle())).To(Succeed())
			})
		})
	})

	Describe("--persistentImage", func() {
		BeforeEach(func() {
			args = []string{"--graph-cleanup-threshold-in-megabytes", "0"}
		})

		Context("when set", func() {
			JustBeforeEach(func() {
				Eventually(client, "30s").Should(gbytes.Say("retain.retained"))
			})

			Context("and local images are used", func() {
				BeforeEach(func() {
					persistentImages = []string{os.Getenv("GARDEN_TEST_ROOTFS")}
				})

				Describe("graph cleanup for a rootfs on the whitelist", func() {
					It("keeps the rootfs in the graph", func() {
						container, err := client.Create(garden.ContainerSpec{
							RootFSPath: persistentImages[0],
						})
						Expect(err).ToNot(HaveOccurred())
						Expect(client.Destroy(container.Handle())).To(Succeed())

						expectLayerCountAfterGraphCleanupToBe(2)
					})

					Context("which is a symlink", func() {
						BeforeEach(func() {
							Expect(os.MkdirAll("/var/vcap/packages", 0755)).To(Succeed())
							err := exec.Command("ln", "-s", os.Getenv("GARDEN_TEST_ROOTFS"), "/var/vcap/packages/busybox").Run()
							Expect(err).ToNot(HaveOccurred())

							persistentImages = []string{"/var/vcap/packages/busybox"}
						})

						AfterEach(func() {
							Expect(os.RemoveAll("/var/vcap/packages")).To(Succeed())
						})

						It("keeps the rootfs in the graph", func() {
							container, err := client.Create(garden.ContainerSpec{
								RootFSPath: persistentImages[0],
							})
							Expect(err).ToNot(HaveOccurred())
							Expect(client.Destroy(container.Handle())).To(Succeed())

							expectLayerCountAfterGraphCleanupToBe(2)
						})
					})
				})

				Describe("graph cleanup for a rootfs not on the whitelist", func() {
					It("cleans up all unused images from the graph", func() {
						container, err := client.Create(garden.ContainerSpec{
							RootFSPath: nonDefaultRootfsPath,
						})
						Expect(err).ToNot(HaveOccurred())
						Expect(client.Destroy(container.Handle())).To(Succeed())

						expectLayerCountAfterGraphCleanupToBe(0)
					})
				})
			})

			Context("and docker images are used", func() {
				BeforeEach(func() {
					persistentImages = []string{
						"docker:///busybox",
						"docker:///ubuntu",
						"docker://banana/bananatest",
					}
				})

				Describe("graph cleanup for a rootfs on the whitelist", func() {
					It("keeps the rootfs in the graph", func() {
						numLayersBeforeDockerPull := numLayersInGraph()
						container, err := client.Create(garden.ContainerSpec{
							RootFSPath: persistentImages[0],
						})
						Expect(err).ToNot(HaveOccurred())
						Expect(client.Destroy(container.Handle())).To(Succeed())
						numLayersInImage := numLayersInGraph() - numLayersBeforeDockerPull

						expectLayerCountAfterGraphCleanupToBe(numLayersInImage)
					})
				})

				Describe("graph cleanup for a rootfs not on the whitelist", func() {
					It("cleans up all unused images from the graph", func() {
						container, err := client.Create(garden.ContainerSpec{
							RootFSPath: "docker:///cfgarden/garden-busybox",
						})
						Expect(err).ToNot(HaveOccurred())
						Expect(client.Destroy(container.Handle())).To(Succeed())

						expectLayerCountAfterGraphCleanupToBe(0)
					})
				})
			})
		})

		Context("when it is not set", func() {
			BeforeEach(func() {
				persistentImages = []string{}
			})

			It("cleans up all unused images from the graph", func() {
				defaultRootfsContainer, err := client.Create(garden.ContainerSpec{})
				Expect(err).ToNot(HaveOccurred())

				nonDefaultRootfsContainer, err := client.Create(garden.ContainerSpec{
					RootFSPath: nonDefaultRootfsPath,
				})
				Expect(err).ToNot(HaveOccurred())

				Expect(client.Destroy(defaultRootfsContainer.Handle())).To(Succeed())
				Expect(client.Destroy(nonDefaultRootfsContainer.Handle())).To(Succeed())

				expectLayerCountAfterGraphCleanupToBe(0)
			})
		})
	})
})

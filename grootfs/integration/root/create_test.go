package root_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/store"

	"github.com/alecthomas/units"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Create", func() {
	var (
		sourceImagePath string
		baseImagePath   string
		rootUID         int
		rootGID         int
		storePath       string
	)

	BeforeEach(func() {
		rootUID = 0
		rootGID = 0

		var err error
		sourceImagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Chown(sourceImagePath, rootUID, rootGID)).To(Succeed())
		Expect(os.Chmod(sourceImagePath, 0755)).To(Succeed())

		storePath, err = ioutil.TempDir(StorePath, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Chmod(storePath, 0777)).To(Succeed())
		Expect(os.Chown(storePath, int(GrootUID), int(GrootGID))).To(Succeed())

		grootFilePath := path.Join(sourceImagePath, "foo")
		Expect(ioutil.WriteFile(grootFilePath, []byte("hello-world"), 0644)).To(Succeed())
		Expect(os.Chown(grootFilePath, int(GrootUID), int(GrootGID))).To(Succeed())

		grootFolder := path.Join(sourceImagePath, "groot-folder")
		Expect(os.Mkdir(grootFolder, 0777)).To(Succeed())
		Expect(os.Chown(grootFolder, int(GrootUID), int(GrootGID))).To(Succeed())
		Expect(ioutil.WriteFile(path.Join(grootFolder, "hello"), []byte("hello-world"), 0644)).To(Succeed())

		rootFilePath := path.Join(sourceImagePath, "bar")
		Expect(ioutil.WriteFile(rootFilePath, []byte("hello-world"), 0644)).To(Succeed())

		rootFolder := path.Join(sourceImagePath, "root-folder")
		Expect(os.Mkdir(rootFolder, 0777)).To(Succeed())
		Expect(ioutil.WriteFile(path.Join(rootFolder, "hello"), []byte("hello-world"), 0644)).To(Succeed())
	})

	JustBeforeEach(func() {
		baseImageFile := integration.CreateBaseImageTar(sourceImagePath)
		baseImagePath = baseImageFile.Name()
	})

	AfterEach(func() {
		Expect(os.RemoveAll(baseImagePath)).To(Succeed())
		Expect(os.RemoveAll(sourceImagePath)).To(Succeed())
	})

	It("keeps the ownership and permissions", func() {
		image := integration.CreateImage(GrootFSBin, storePath, DraxBin, baseImagePath, "random-id", 0)

		grootFi, err := os.Stat(path.Join(image.RootFSPath, "foo"))
		Expect(err).NotTo(HaveOccurred())
		Expect(grootFi.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID)))
		Expect(grootFi.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID)))

		rootFi, err := os.Stat(path.Join(image.RootFSPath, "bar"))
		Expect(err).NotTo(HaveOccurred())
		Expect(rootFi.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(rootUID)))
		Expect(rootFi.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(rootGID)))
	})

	Context("when mappings are provided", func() {
		// This test is in the root suite not because `grootfs` is run by root, but
		// because we need to write a file as root to test the translation.
		It("translates the rootfs accordingly", func() {
			cmd := exec.Command(
				GrootFSBin, "--store", storePath,
				"--log-level", "debug",
				"create",
				"--uid-mapping", fmt.Sprintf("0:%d:1", GrootUID),
				"--uid-mapping", "1:100000:65000",
				"--gid-mapping", fmt.Sprintf("0:%d:1", GrootGID),
				"--gid-mapping", "1:100000:65000",
				baseImagePath,
				"some-id",
			)
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: GrootUID,
					Gid: GrootGID,
				},
			}
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
			image := strings.TrimSpace(string(sess.Out.Contents()))

			grootFi, err := os.Stat(path.Join(image, "rootfs", "foo"))
			Expect(err).NotTo(HaveOccurred())
			Expect(grootFi.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID + 99999)))
			Expect(grootFi.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID + 99999)))

			grootDir, err := os.Stat(path.Join(image, "rootfs", "groot-folder"))
			Expect(err).NotTo(HaveOccurred())
			Expect(grootDir.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID + 99999)))
			Expect(grootDir.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID + 99999)))

			rootFi, err := os.Stat(path.Join(image, "rootfs", "bar"))
			Expect(err).NotTo(HaveOccurred())
			Expect(rootFi.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID)))
			Expect(rootFi.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID)))

			rootDir, err := os.Stat(path.Join(image, "rootfs", "root-folder"))
			Expect(err).NotTo(HaveOccurred())
			Expect(rootDir.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID)))
			Expect(rootDir.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID)))
		})

		Context("and it's executed as root", func() {
			It("translates the rootfs accordingly", func() {
				cmd := exec.Command(
					GrootFSBin, "--store", storePath,
					"--log-level", "debug",
					"create",
					"--uid-mapping", fmt.Sprintf("0:%d:1", GrootUID),
					"--uid-mapping", "1:100000:65000",
					"--gid-mapping", fmt.Sprintf("0:%d:1", GrootGID),
					"--gid-mapping", "1:100000:65000",
					baseImagePath,
					"some-id",
				)

				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))
				image := strings.TrimSpace(string(sess.Out.Contents()))

				grootFi, err := os.Stat(path.Join(image, "rootfs", "foo"))
				Expect(err).NotTo(HaveOccurred())
				Expect(grootFi.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID + 99999)))
				Expect(grootFi.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID + 99999)))

				grootDir, err := os.Stat(path.Join(image, "rootfs", "groot-folder"))
				Expect(err).NotTo(HaveOccurred())
				Expect(grootDir.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID + 99999)))
				Expect(grootDir.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID + 99999)))

				rootFi, err := os.Stat(path.Join(image, "rootfs", "bar"))
				Expect(err).NotTo(HaveOccurred())
				Expect(rootFi.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID)))
				Expect(rootFi.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID)))

				rootDir, err := os.Stat(path.Join(image, "rootfs", "root-folder"))
				Expect(err).NotTo(HaveOccurred())
				Expect(rootDir.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID)))
				Expect(rootDir.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID)))
			})

			It("allows the mapped user to have access to the created image", func() {
				cmd := exec.Command(
					GrootFSBin, "--store", storePath,
					"--log-level", "debug",
					"create",
					"--uid-mapping", fmt.Sprintf("0:%d:1", GrootUID),
					"--uid-mapping", "1:100000:65000",
					"--gid-mapping", fmt.Sprintf("0:%d:1", GrootGID),
					"--gid-mapping", "1:100000:65000",
					baseImagePath,
					"some-id",
				)

				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))
				image := strings.TrimSpace(string(sess.Out.Contents()))

				listRootfsCmd := exec.Command("ls", filepath.Join(image, "rootfs", "root-folder"))
				listRootfsCmd.SysProcAttr = &syscall.SysProcAttr{
					Credential: &syscall.Credential{
						Uid: GrootUID,
						Gid: GrootGID,
					},
				}

				sess, err = gexec.Start(listRootfsCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))
			})
		})
	})

	Context("when image is local", func() {
		It("logs the steps taken to create the rootfs", func() {
			cmd := exec.Command(
				GrootFSBin, "--store", storePath,
				"--log-level", "debug",
				"create",
				"--uid-mapping", fmt.Sprintf("0:%d:1", GrootUID),
				"--uid-mapping", "1:100000:65000",
				"--gid-mapping", fmt.Sprintf("0:%d:1", GrootGID),
				"--gid-mapping", "1:100000:65000",
				baseImagePath,
				"some-id",
			)
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: GrootUID,
					Gid: GrootGID,
				},
			}
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess, 10*time.Second).Should(gexec.Exit(0))

			Eventually(sess.Err).Should(gbytes.Say("grootfs.create.groot-creating.image-pulling.ns-id-mapper-unpacking.starting-unpack-wrapper-command"))
			Eventually(sess.Err).Should(gbytes.Say("grootfs.create.groot-creating.image-pulling.ns-id-mapper-unpacking.mapUID.starting-id-map"))
			Eventually(sess.Err).Should(gbytes.Say("grootfs.create.groot-creating.image-pulling.ns-id-mapper-unpacking.mapGID.starting-id-map"))
		})
	})

	Context("when image is remote", func() {
		It("logs the steps taken to create the rootfs", func() {
			cmd := exec.Command(
				GrootFSBin, "--store", storePath,
				"--log-level", "debug",
				"create",
				"--uid-mapping", fmt.Sprintf("0:%d:1", GrootUID),
				"--uid-mapping", "1:100000:65000",
				"--gid-mapping", fmt.Sprintf("0:%d:1", GrootGID),
				"--gid-mapping", "1:100000:65000",
				"docker:///cfgarden/empty:v0.1.0",
				"some-id",
			)
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: GrootUID,
					Gid: GrootGID,
				},
			}
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess, 10*time.Second).Should(gexec.Exit(0))

			Eventually(sess.Err).Should(gbytes.Say("grootfs.create.groot-creating.image-pulling.btrfs-creating-volume.starting-btrfs"))
			Eventually(sess.Err).Should(gbytes.Say("grootfs.create.groot-creating.image-pulling.ns-id-mapper-unpacking.starting-unpack-wrapper-command"))
			Eventually(sess.Err).Should(gbytes.Say("grootfs.create.groot-creating.image-pulling.ns-id-mapper-unpacking.mapUID.starting-id-map"))
			Eventually(sess.Err).Should(gbytes.Say("grootfs.create.groot-creating.image-pulling.ns-id-mapper-unpacking.mapGID.starting-id-map"))
			Eventually(sess.Err).Should(gbytes.Say("grootfs.create.groot-creating.image-pulling.ns-id-mapper-unpacking.unpack-wrapper.starting-unpack"))
			Eventually(sess.Err).Should(gbytes.Say("grootfs.create.groot-creating.making-image.btrfs-creating-snapshot.starting-btrfs"))
		})

		Context("when the image is bigger than available memory", func() {
			It("doesn't fail", func() {
				cmd := exec.Command(
					GrootFSBin,
					"--store", storePath,
					"--log-level", "fatal",
					"create",
					"docker:///ubuntu:trusty",
					"some-id",
				)

				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				go func() {
					defer GinkgoRecover()

					statsPath := path.Join("/proc", strconv.Itoa(sess.Command.Process.Pid), "status")
					runs := 0
					for {
						stats, err := ioutil.ReadFile(statsPath)
						if err != nil {
							Expect(runs).To(BeNumerically(">", 1))
							break
						}

						var statsMap map[string]string
						Expect(yaml.Unmarshal(stats, &statsMap)).To(Succeed())

						n, err := units.ParseBase2Bytes(strings.Replace(strings.ToUpper(statsMap["VmHWM"]), " ", "", -1))
						Expect(err).NotTo(HaveOccurred())
						// Biggest ubuntu:trusty layer is 65694192 bytes
						Expect(n).To(BeNumerically("<", 50*1024*1024))

						time.Sleep(200 * time.Millisecond)
						runs++
					}
				}()

				Eventually(sess, 45*time.Second).Should(gexec.Exit(0))
			})
		})
	})

	Context("store configuration", func() {
		Context("when there's no mapping", func() {
			It("sets the onwership of the store to the caller user", func() {
				cmd := exec.Command(
					GrootFSBin, "--store", storePath,
					"--log-level", "debug",
					"create",
					baseImagePath,
					"some-id",
				)
				cmd.SysProcAttr = &syscall.SysProcAttr{
					Credential: &syscall.Credential{
						Uid: GrootUID,
						Gid: GrootGID,
					},
				}

				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess, 10*time.Second).Should(gexec.Exit(0))

				stat, err := os.Stat(filepath.Join(storePath, store.IMAGES_DIR_NAME))
				Expect(err).NotTo(HaveOccurred())
				Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID)))
				Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID)))

				stat, err = os.Stat(filepath.Join(storePath, store.VOLUMES_DIR_NAME))
				Expect(err).NotTo(HaveOccurred())
				Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID)))
				Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID)))

				stat, err = os.Stat(filepath.Join(storePath, store.LOCKS_DIR_NAME))
				Expect(err).NotTo(HaveOccurred())
				Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID)))
				Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID)))
			})
		})

		Context("when there's mappings", func() {
			It("sets the onwnership of the store to the mapped user", func() {
				cmd := exec.Command(
					GrootFSBin, "--store", storePath,
					"--log-level", "debug",
					"create",
					"--uid-mapping", fmt.Sprintf("0:%d:1", 5000),
					"--uid-mapping", "1:100000:65000",
					"--gid-mapping", fmt.Sprintf("0:%d:1", 6000),
					"--gid-mapping", "1:100000:65000",
					baseImagePath,
					"some-id",
				)

				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				// Fails because these mappings aren't valid
				Eventually(sess, 10*time.Second).Should(gexec.Exit(1))

				stat, err := os.Stat(filepath.Join(storePath, store.IMAGES_DIR_NAME))
				Expect(err).NotTo(HaveOccurred())
				Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(5000)))
				Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(6000)))

				stat, err = os.Stat(filepath.Join(storePath, store.VOLUMES_DIR_NAME))
				Expect(err).NotTo(HaveOccurred())
				Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(5000)))
				Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(6000)))

				stat, err = os.Stat(filepath.Join(storePath, store.LOCKS_DIR_NAME))
				Expect(err).NotTo(HaveOccurred())
				Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(5000)))
				Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(6000)))
			})

			Context("but there's no mapping for root or size = 1", func() {
				It("fails fast", func() {
					cmd := exec.Command(
						GrootFSBin, "--store", storePath,
						"--log-level", "debug",
						"create",
						"--uid-mapping", "1:100000:65000",
						"--gid-mapping", "1:100000:65000",
						baseImagePath,
						"some-id",
					)

					sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(1))
					Eventually(sess).Should(gbytes.Say("couldn't determine store owner, missing root user mapping"))
				})
			})
		})
	})
})

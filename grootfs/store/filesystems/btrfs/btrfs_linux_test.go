package btrfs_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/filesystems/btrfs"
	"code.cloudfoundry.org/grootfs/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	. "github.com/st3v/glager"
)

var _ = Describe("Btrfs", func() {
	const btrfsMountPath = "/mnt/btrfs"

	var (
		driver      *btrfs.Btrfs
		logger      *TestLogger
		storeName   string
		storePath   string
		draxBinPath string
		volumesPath string
	)

	BeforeEach(func() {
		var err error
		storeName = fmt.Sprintf("test-store-%d", GinkgoParallelNode())
		Expect(os.MkdirAll(filepath.Join(btrfsMountPath, storeName), 0755)).To(Succeed())
		storePath, err = ioutil.TempDir(filepath.Join(btrfsMountPath, storeName), "")
		Expect(err).NotTo(HaveOccurred())

		volumesPath = filepath.Join(storePath, store.VOLUMES_DIR_NAME)
		Expect(os.MkdirAll(volumesPath, 0755)).To(Succeed())

		draxBinPath, err = gexec.Build("code.cloudfoundry.org/grootfs/store/filesystems/btrfs/drax")
		Expect(err).NotTo(HaveOccurred())
		testhelpers.SuidDrax(draxBinPath)

		logger = NewLogger("btrfs")
	})

	JustBeforeEach(func() {
		driver = btrfs.NewBtrfs("btrfs", draxBinPath, storePath)
	})

	AfterEach(func() {
		testhelpers.CleanUpSubvolumes(btrfsMountPath, storeName)
		gexec.CleanupBuildArtifacts()
	})

	Describe("Path", func() {
		It("returns the volume path when it exists", func() {
			volID := randVolumeID()
			volPath, err := driver.Create(logger, "", volID)
			Expect(err).NotTo(HaveOccurred())

			retVolPath, err := driver.Path(logger, volID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retVolPath).To(Equal(volPath))
		})

		Context("when the volume does not exist", func() {
			It("returns an error", func() {
				_, err := driver.Path(logger, "non-existent-id")
				Expect(err).To(MatchError(ContainSubstring("volume does not exist")))
			})
		})
	})

	Describe("Create", func() {
		Context("when the parent is empty", func() {
			It("creates a BTRFS subvolume", func() {
				volID := randVolumeID()
				volPath, err := driver.Create(logger, "", volID)
				Expect(err).NotTo(HaveOccurred())

				Expect(volPath).To(BeADirectory())
			})

			It("logs the correct btrfs command", func() {
				volID := randVolumeID()
				volumePath, err := driver.Create(logger, "", volID)
				Expect(err).NotTo(HaveOccurred())

				Expect(logger).To(ContainSequence(
					Debug(
						Message("btrfs.btrfs-creating-volume.starting-btrfs"),
						Data("path", "/bin/btrfs"),
						Data("args", []string{"btrfs", "subvolume", "create", volumePath}),
						Data("id", volID),
					),
				))
			})
		})

		Context("custom btrfs binary path", func() {
			It("uses the custom btrfs binary given", func() {
				driver = btrfs.NewBtrfs("cool-btrfs", draxBinPath, storePath)
				_, err := driver.Create(logger, "", "random-id")
				Expect(err).To(MatchError(ContainSubstring(`"cool-btrfs": executable file not found in $PATH`)))
			})
		})

		Context("when the parent is not empty", func() {
			It("creates a driver.snapshot", func() {
				volumeID := randVolumeID()
				destVolID := randVolumeID()

				fromPath, err := driver.Create(logger, "", volumeID)
				Expect(err).NotTo(HaveOccurred())

				Expect(ioutil.WriteFile(filepath.Join(fromPath, "a_file"), []byte("hello-world"), 0666)).To(Succeed())

				destVolPath, err := driver.Create(logger, volumeID, destVolID)
				Expect(err).NotTo(HaveOccurred())

				Expect(filepath.Join(destVolPath, "a_file")).To(BeARegularFile())
			})

			It("logs the correct btrfs command", func() {
				volumeID := randVolumeID()
				destVolID := randVolumeID()

				fromPath, err := driver.Create(logger, "", volumeID)
				Expect(err).NotTo(HaveOccurred())

				destVolPath, err := driver.Create(logger, volumeID, destVolID)
				Expect(err).NotTo(HaveOccurred())

				Expect(logger).To(ContainSequence(
					Debug(
						Message("btrfs.btrfs-creating-volume.starting-btrfs"),
						Data("path", "/bin/btrfs"),
						Data("args", []string{"btrfs", "subvolume", "snapshot", fromPath, destVolPath}),
						Data("id", destVolID),
						Data("parentID", volumeID),
					),
				))
			})
		})

		Context("when the volume exists", func() {
			It("returns an error", func() {
				volID := randVolumeID()
				volPath := filepath.Join(storePath, store.VOLUMES_DIR_NAME, volID)
				Expect(os.MkdirAll(volPath, 0777)).To(Succeed())

				_, err := driver.Create(logger, "", volID)
				Expect(err).To(MatchError(ContainSubstring("creating btrfs volume")))
			})
		})

		Context("when the parent does not exist", func() {
			It("returns an error", func() {
				volID := randVolumeID()

				_, err := driver.Create(logger, "non-existent-parent", volID)
				Expect(err).To(MatchError(ContainSubstring("creating btrfs volume")))
			})
		})
	})

	Describe("Snapshot", func() {
		var toPath string

		BeforeEach(func() {
			imagePath, err := ioutil.TempDir(storePath, "")
			Expect(err).NotTo(HaveOccurred())
			toPath = filepath.Join(imagePath, "rootfs")
		})

		It("creates a driver.snapshot", func() {
			volumeID := randVolumeID()

			fromPath, err := driver.Create(logger, "", volumeID)
			Expect(err).NotTo(HaveOccurred())

			Expect(ioutil.WriteFile(filepath.Join(fromPath, "a_file"), []byte("hello-world"), 0666)).To(Succeed())

			Expect(driver.Snapshot(logger, fromPath, toPath)).To(Succeed())

			Expect(filepath.Join(toPath, "a_file")).To(BeARegularFile())
		})

		It("logs the correct btrfs command", func() {
			volumeID := randVolumeID()

			fromPath, err := driver.Create(logger, "", volumeID)
			Expect(err).NotTo(HaveOccurred())

			Expect(ioutil.WriteFile(filepath.Join(fromPath, "a_file"), []byte("hello-world"), 0666)).To(Succeed())

			Expect(driver.Snapshot(logger, fromPath, toPath)).To(Succeed())

			Expect(logger).To(ContainSequence(
				Debug(
					Message("btrfs.btrfs-creating-snapshot.starting-btrfs"),
					Data("path", "/bin/btrfs"),
					Data("fromPath", fromPath),
					Data("toPath", toPath),
					Data("args", []string{"btrfs", "subvolume", "snapshot", fromPath, toPath}),
				),
			))
		})

		Context("custom btrfs binary path", func() {
			It("uses the custom btrfs binary given", func() {
				driver = btrfs.NewBtrfs("cool-btrfs", draxBinPath, storePath)
				err := driver.Snapshot(logger, "from", "to")
				Expect(err).To(MatchError(ContainSubstring(`"cool-btrfs": executable file not found in $PATH`)))
			})
		})

		Context("when the parent does not exist", func() {
			It("returns an error", func() {
				Expect(
					driver.Snapshot(logger, "non-existent-parent", toPath),
				).To(MatchError(ContainSubstring("creating btrfs snapshot")))
			})
		})
	})

	Describe("Volumes", func() {
		BeforeEach(func() {
			Expect(os.Mkdir(filepath.Join(volumesPath, "sha256:vol-a"), 0777)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(volumesPath, "sha256:vol-b"), 0777)).To(Succeed())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(volumesPath)).To(Succeed())
		})

		It("returns a list with existing volumes id", func() {
			volumes, err := driver.Volumes(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(volumes)).To(Equal(2))
			Expect(volumes).To(ContainElement("sha256:vol-a"))
			Expect(volumes).To(ContainElement("sha256:vol-b"))
		})

		Context("when fails to list volumes", func() {
			BeforeEach(func() {
				Expect(os.Chmod(volumesPath, 0666)).To(Succeed())
			})

			AfterEach(func() {
				Expect(os.Chmod(volumesPath, 0755)).To(Succeed())
			})

			It("returns an error", func() {
				_, err := driver.Volumes(logger)
				Expect(err).To(MatchError(ContainSubstring("failed to list volumes")))
			})
		})
	})

	Describe("DestroyVolume", func() {
		var (
			volumeID   string
			volumePath string
		)

		JustBeforeEach(func() {
			volumeID = randVolumeID()
			var err error
			volumePath, err = driver.Create(logger, "", volumeID)
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes the btrfs volume by id", func() {
			Expect(volumePath).To(BeADirectory())

			Expect(driver.DestroyVolume(logger, volumeID)).To(Succeed())
			Expect(volumePath).ToNot(BeAnExistingFile())
		})

		Context("custom btrfs binary path", func() {
			It("uses the custom btrfs binary given", func() {
				driver = btrfs.NewBtrfs("cool-btrfs", draxBinPath, storePath)
				err := driver.DestroyVolume(logger, volumeID)
				Expect(err).To(MatchError(ContainSubstring(`"cool-btrfs": executable file not found in $PATH`)))
			})
		})
	})

	Describe("Destroy", func() {
		var volumePath string

		Context("when a volume exists", func() {
			JustBeforeEach(func() {
				var err error
				volumePath, err = driver.Create(logger, "", randVolumeID())
				Expect(err).NotTo(HaveOccurred())
			})

			It("deletes the btrfs volume", func() {
				Expect(volumePath).To(BeADirectory())

				Expect(driver.Destroy(logger, volumePath)).To(Succeed())
				Expect(volumePath).ToNot(BeAnExistingFile())
			})

			It("deletes the quota group for the volume", func() {
				rootIDBuffer := gbytes.NewBuffer()
				sess, err := gexec.Start(exec.Command("sudo", "btrfs", "inspect-internal", "rootid", volumePath), rootIDBuffer, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))
				rootID := strings.TrimSpace(string(rootIDBuffer.Contents()))

				sess, err = gexec.Start(exec.Command("sudo", "btrfs", "qgroup", "show", storePath), GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess).To(gbytes.Say(rootID))

				Expect(driver.Destroy(logger, volumePath)).To(Succeed())

				sess, err = gexec.Start(exec.Command("sudo", "btrfs", "qgroup", "show", storePath), GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess).ToNot(gbytes.Say(rootID))
			})

			It("logs the correct btrfs command", func() {
				Expect(volumePath).To(BeADirectory())

				Expect(driver.Destroy(logger, volumePath)).To(Succeed())

				Expect(logger).To(ContainSequence(
					Debug(
						Message("btrfs.btrfs-destroying.starting-btrfs"),
						Data("path", "/bin/btrfs"),
						Data("args", []string{"btrfs", "subvolume", "delete", volumePath}),
					),
				))
			})

			Context("custom btrfs binary path", func() {
				It("uses the custom btrfs binary given", func() {
					driver = btrfs.NewBtrfs("cool-btrfs", draxBinPath, storePath)
					err := driver.Destroy(logger, volumePath)
					Expect(err).To(MatchError(ContainSubstring(`"cool-btrfs": executable file not found in $PATH`)))
				})
			})

			Context("and drax does not exist", func() {
				BeforeEach(func() {
					draxBinPath = "/path/to/non-existent-drax"
				})

				It("succeeds", func() {
					Expect(driver.Destroy(logger, volumePath)).To(Succeed())
				})
			})

			Context("and drax does not have the setuid bit", func() {
				BeforeEach(func() {
					testhelpers.UnsuidDrax(draxBinPath)
				})

				It("doesn't fail, but logs the error", func() {
					Expect(
						driver.Destroy(logger, volumePath),
					).To(Succeed())

					Expect(logger).To(ContainSequence(
						Error(
							errors.New("missing the setuid bit on drax"),
							Message("btrfs.btrfs-destroying.destroying-quota-groups-failed"),
						),
					))
				})
			})
		})

		Context("when destroying a non existant volume", func() {
			It("returns an error", func() {
				err := driver.Destroy(logger, "non-existant")

				Expect(err).To(MatchError(ContainSubstring("image path not found")))
			})
		})

		Context("when destroying the volume fails", func() {
			It("returns an error", func() {
				tmpDir, _ := ioutil.TempDir("", "")
				err := driver.Destroy(logger, tmpDir)

				Expect(err).To(MatchError(ContainSubstring("destroying volume")))
			})
		})
	})

	Describe("ApplyDiskLimit", func() {
		var snapshotPath string

		BeforeEach(func() {
			imagePath, err := ioutil.TempDir(storePath, "")
			Expect(err).NotTo(HaveOccurred())
			snapshotPath = filepath.Join(imagePath, "rootfs")
		})

		Context("when the snapshot path is a volume", func() {
			JustBeforeEach(func() {
				volID := randVolumeID()
				volumePath, err := driver.Create(logger, "", volID)
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(volumePath, "vol-file")), "bs=1048576", "count=5")
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				Expect(driver.Snapshot(logger, volumePath, snapshotPath)).To(Succeed())
			})

			It("applies the disk limit", func() {
				Expect(driver.ApplyDiskLimit(logger, snapshotPath, 10*1024*1024, false)).To(Succeed())

				cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(snapshotPath, "hello")), "bs=1048576", "count=4")
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				cmd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(snapshotPath, "hello2")), "bs=1048576", "count=2")
				sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(1))
				Expect(sess.Err).To(gbytes.Say("Disk quota exceeded"))
			})

			Context("when using a custom btrfs binary", func() {
				var (
					btrfsCalledFile *os.File
					btrfsBin        *os.File
					tempFolder      string
				)

				BeforeEach(func() {
					tempFolder, btrfsBin, btrfsCalledFile = integration.CreateFakeBin("btrfs")
				})

				AfterEach(func() {
					Expect(os.RemoveAll(tempFolder)).To(Succeed())
				})

				It("will force drax to use that binary", func() {
					driver = btrfs.NewBtrfs(btrfsBin.Name(), draxBinPath, storePath)
					Expect(driver.ApplyDiskLimit(logger, snapshotPath, 10*1024*1024, true)).To(Succeed())
					contents, err := ioutil.ReadFile(btrfsCalledFile.Name())
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("I'm groot - btrfs"))
				})
			})

			Context("when exclusive limit is active", func() {
				It("applies the disk limit with the exclusive flag", func() {
					Expect(driver.ApplyDiskLimit(logger, snapshotPath, 10*1024*1024, true)).To(Succeed())

					cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(snapshotPath, "hello")), "bs=1048576", "count=6")
					sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))

					cmd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(snapshotPath, "hello2")), "bs=1048576", "count=5")
					sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(1))
					Expect(sess.Err).To(gbytes.Say("Disk quota exceeded"))
				})
			})

			Context("when drax does not exist", func() {
				BeforeEach(func() {
					draxBinPath = "/path/to/non-existent-drax"
				})

				It("returns an error", func() {
					Expect(
						driver.ApplyDiskLimit(logger, snapshotPath, 10*1024*1024, false),
					).To(MatchError(ContainSubstring("drax was not found in the $PATH")))
				})
			})

			Context("and drax does not have the setuid bit", func() {
				BeforeEach(func() {
					testhelpers.UnsuidDrax(draxBinPath)
				})

				It("returns an error", func() {
					Expect(
						driver.ApplyDiskLimit(logger, snapshotPath, 10*1024*1024, false),
					).To(MatchError(ContainSubstring("missing the setuid bit on drax")))
				})
			})
		})

		Context("when the provided path is not a volume", func() {
			It("returns an error", func() {
				Expect(os.MkdirAll(snapshotPath, 0777)).To(Succeed())

				Expect(
					driver.ApplyDiskLimit(logger, snapshotPath, 10*1024*1024, false),
				).To(MatchError(ContainSubstring("is not a subvolume")))
			})
		})
	})

	Describe("FetchStats", func() {
		var toPath string

		BeforeEach(func() {
			imagePath, err := ioutil.TempDir(storePath, "")
			Expect(err).NotTo(HaveOccurred())
			toPath = filepath.Join(imagePath, "rootfs")
		})

		Context("when the provided path is a volume", func() {
			JustBeforeEach(func() {
				volID := randVolumeID()
				volPath, err := driver.Create(logger, "", volID)
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(volPath, "vol-file")), "bs=4210688", "count=1")
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				Expect(driver.Snapshot(logger, volPath, toPath)).To(Succeed())
			})

			It("returns the correct stats", func() {
				cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(toPath, "hello")), "bs=4210688", "count=1")
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				stats, err := driver.FetchStats(logger, toPath)
				Expect(err).ToNot(HaveOccurred())

				// Block math craziness -> 1* 4210688 ~= 4227072
				Expect(stats.DiskUsage.ExclusiveBytesUsed).To(Equal(int64(4227072)))
				// Block math craziness -> 2* 4227072 ~= 8437760
				Expect(stats.DiskUsage.TotalBytesUsed).To(Equal(int64(8437760)))
			})

			Context("when using a custom btrfs binary", func() {
				var (
					btrfsCalledFile *os.File
					btrfsBin        *os.File
					tempFolder      string
				)

				BeforeEach(func() {
					tempFolder, btrfsBin, btrfsCalledFile = integration.CreateFakeBin("btrfs")
				})

				AfterEach(func() {
					Expect(os.RemoveAll(tempFolder)).To(Succeed())
				})

				It("will force drax to use that binary", func() {
					driver = btrfs.NewBtrfs(btrfsBin.Name(), draxBinPath, storePath)
					_, err := driver.FetchStats(logger, toPath)
					Expect(err).To(HaveOccurred())

					contents, err := ioutil.ReadFile(btrfsCalledFile.Name())
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("I'm groot - btrfs"))
				})
			})

			Context("when drax does not exist", func() {
				BeforeEach(func() {
					draxBinPath = "/path/to/non-existent-drax"
				})

				It("returns an error", func() {
					_, err := driver.FetchStats(logger, toPath)
					Expect(err).To(MatchError(ContainSubstring("drax was not found in the $PATH")))
				})
			})

			Context("and drax does not have the setuid bit", func() {
				BeforeEach(func() {
					testhelpers.UnsuidDrax(draxBinPath)
				})

				It("returns an error", func() {
					_, err := driver.FetchStats(logger, toPath)
					Expect(err).To(MatchError(ContainSubstring("missing the setuid bit on drax")))
				})
			})
		})

		Context("when the provided path is not a volume", func() {
			BeforeEach(func() {
				Expect(os.MkdirAll(toPath, 0777)).To(Succeed())
			})

			It("returns an error", func() {
				_, err := driver.FetchStats(logger, toPath)
				Expect(err).To(MatchError(ContainSubstring("is not a btrfs volume")))
			})
		})

		Context("when path does not exist", func() {
			BeforeEach(func() {
				toPath = "/tmp/not-here"
			})

			It("returns an error", func() {
				_, err := driver.FetchStats(logger, toPath)
				Expect(err).To(MatchError(ContainSubstring("No such file or directory")))
			})
		})
	})
})

func randVolumeID() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("volume-%d", r.Int())
}

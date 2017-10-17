package btrfs_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/filesystems"
	"code.cloudfoundry.org/grootfs/store/filesystems/btrfs"
	"code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/grootfs/testhelpers"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	. "github.com/st3v/glager"
)

var _ = Describe("Btrfs", func() {
	const storeName = "test-store"

	var (
		driver         *btrfs.Driver
		logger         *TestLogger
		storePath      string
		draxBinPath    string
		volumesPath    string
		metaPath       string
		btrfsMountPath string
		randomImageID  string
	)

	BeforeEach(func() {
		btrfsMountPath = fmt.Sprintf("/mnt/btrfs-%d", GinkgoParallelNode())

		var err error
		Expect(os.MkdirAll(filepath.Join(btrfsMountPath, storeName), 0755)).To(Succeed())
		storePath, err = ioutil.TempDir(filepath.Join(btrfsMountPath, storeName), "")
		Expect(err).NotTo(HaveOccurred())

		volumesPath = filepath.Join(storePath, store.VolumesDirName)
		Expect(os.MkdirAll(volumesPath, 0755)).To(Succeed())

		metaPath = filepath.Join(storePath, store.MetaDirName)
		Expect(os.MkdirAll(metaPath, 0755)).To(Succeed())

		draxBinPath, err = gexec.Build("code.cloudfoundry.org/grootfs/store/filesystems/btrfs/drax")
		Expect(err).NotTo(HaveOccurred())
		testhelpers.SuidBinary(draxBinPath)

		randomImageID = testhelpers.NewRandomID()

		logger = NewLogger("btrfs")
	})

	JustBeforeEach(func() {
		driver = btrfs.NewDriver("btrfs", "mkfs.btrfs", draxBinPath, storePath)
	})

	AfterEach(func() {
		testhelpers.CleanUpBtrfsSubvolumes(btrfsMountPath)
		gexec.CleanupBuildArtifacts()
	})

	Describe("InitFilesystem", func() {
		var fsFile, newStorePath string

		BeforeEach(func() {
			tempFile, err := ioutil.TempFile("", "btrfs-filesystem")
			Expect(err).NotTo(HaveOccurred())
			fsFile = tempFile.Name()
			Expect(os.Truncate(fsFile, 1024*1024*1024)).To(Succeed())

			newStorePath, err = ioutil.TempDir("", "store")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			syscall.Unmount(newStorePath, 0)
		})

		It("succcesfully creates and mounts a filesystem", func() {
			Expect(driver.InitFilesystem(logger, fsFile, newStorePath)).To(Succeed())
			statfs := syscall.Statfs_t{}
			Expect(exec.Command("mountpoint", newStorePath).Run()).To(Succeed())
			Expect(syscall.Statfs(newStorePath, &statfs)).To(Succeed())
			Expect(statfs.Type).To(Equal(filesystems.BtrfsType))
		})

		It("successfully mounts the filesystem with the correct mount options", func() {
			Expect(driver.InitFilesystem(logger, fsFile, newStorePath)).To(Succeed())
			mountinfo, err := ioutil.ReadFile("/proc/self/mountinfo")
			Expect(err).NotTo(HaveOccurred())

			Expect(string(mountinfo)).To(MatchRegexp(fmt.Sprintf("%s[^\n]*rw[^\n]*user_subvol_rm_allowed", newStorePath)))
		})

		Context("when creating the filesystem fails", func() {
			It("returns an error", func() {
				err := driver.InitFilesystem(logger, "/tmp/no-valid", newStorePath)
				Expect(err).To(MatchError(ContainSubstring("Formatting BTRFS filesystem")))
			})
		})

		Context("when the filesystem is already formatted", func() {
			BeforeEach(func() {
				cmd := exec.Command("mkfs.btrfs", "-f", fsFile)
				Expect(os.Truncate(fsFile, 200*1024*1024)).To(Succeed())
				Expect(cmd.Run()).To(Succeed())
			})

			It("succeeds", func() {
				Expect(driver.InitFilesystem(logger, fsFile, newStorePath)).To(Succeed())
			})
		})

		Context("when the store is already mounted", func() {
			BeforeEach(func() {
				Expect(os.Truncate(fsFile, 200*1024*1024)).To(Succeed())
				cmd := exec.Command("mkfs.btrfs", "-f", fsFile)
				Expect(cmd.Run()).To(Succeed())
				cmd = exec.Command("mount", "-o", "", "-t", "btrfs", fsFile, newStorePath)
				Expect(cmd.Run()).To(Succeed())
			})

			It("succeeds", func() {
				Expect(driver.InitFilesystem(logger, fsFile, newStorePath)).To(Succeed())
				mountinfo, err := ioutil.ReadFile("/proc/self/mountinfo")
				Expect(err).NotTo(HaveOccurred())

				Expect(string(mountinfo)).To(MatchRegexp(fmt.Sprintf("%s[^\n]*rw[^\n]*user_subvol_rm_allowed", newStorePath)))
			})
		})

		Context("when mounting the filesystem fails", func() {
			It("returns an error", func() {
				err := driver.InitFilesystem(logger, fsFile, "/tmp/no-valid")
				Expect(err).To(MatchError(ContainSubstring("Mounting filesystem")))
			})
		})

		Context("when using a custom mkfs.btrfs binary", func() {
			var (
				mkfsCalledFile *os.File
				mkfsBin        *os.File
				tempFolder     string
			)

			BeforeEach(func() {
				tempFolder, mkfsBin, mkfsCalledFile = integration.CreateFakeBin("mkfs.btrfs")
			})

			AfterEach(func() {
				Expect(os.RemoveAll(tempFolder)).To(Succeed())
			})

			It("will use that binary to format the filesystem", func() {
				driver = btrfs.NewDriver("btrfs", mkfsBin.Name(), draxBinPath, storePath)
				_ = driver.InitFilesystem(logger, fsFile, newStorePath)

				contents, err := ioutil.ReadFile(mkfsCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - mkfs.btrfs"))
			})
		})
	})

	Describe("ValidateFileSystem", func() {
		Context("when storepath is a BTRFS mount", func() {
			It("returns no error", func() {
				Expect(driver.ValidateFileSystem(logger, storePath)).To(Succeed())
			})
		})

		Context("when storepath is not a BTRFS mount", func() {
			It("returns an error", func() {
				err := driver.ValidateFileSystem(logger, "/mnt/ext4")
				Expect(err).To(MatchError(ContainSubstring("Store path filesystem (/mnt/ext4) is incompatible with requested driver")))
			})
		})
	})

	Describe("VolumePath", func() {
		It("returns the volume path when it exists", func() {
			volID := randVolumeID()
			volPath, err := driver.CreateVolume(logger, "", volID)
			Expect(err).NotTo(HaveOccurred())

			retVolPath, err := driver.VolumePath(logger, volID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retVolPath).To(Equal(volPath))
		})

		Context("when the volume does not exist", func() {
			It("returns an error", func() {
				_, err := driver.VolumePath(logger, "non-existent-id")
				Expect(err).To(MatchError(ContainSubstring("volume does not exist")))
			})
		})
	})

	Describe("CreateVolume", func() {
		Context("when the parent is empty", func() {
			It("creates a BTRFS subvolume", func() {
				volID := randVolumeID()
				volPath, err := driver.CreateVolume(logger, "", volID)
				Expect(err).NotTo(HaveOccurred())

				Expect(volPath).To(BeADirectory())
			})

			It("logs the correct btrfs command", func() {
				volID := randVolumeID()
				volumePath, err := driver.CreateVolume(logger, "", volID)
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
				driver = btrfs.NewDriver("cool-btrfs", "mkfs.btrfs", draxBinPath, storePath)
				_, err := driver.CreateVolume(logger, "", randomImageID)
				Expect(err).To(MatchError(ContainSubstring(`"cool-btrfs": executable file not found in $PATH`)))
			})
		})

		Context("when the parent is not empty", func() {
			It("creates a driver.snapshot", func() {
				volumeID := randVolumeID()
				destVolID := randVolumeID()

				fromPath, err := driver.CreateVolume(logger, "", volumeID)
				Expect(err).NotTo(HaveOccurred())

				Expect(ioutil.WriteFile(filepath.Join(fromPath, "a_file"), []byte("hello-world"), 0666)).To(Succeed())

				destVolPath, err := driver.CreateVolume(logger, volumeID, destVolID)
				Expect(err).NotTo(HaveOccurred())

				Expect(filepath.Join(destVolPath, "a_file")).To(BeARegularFile())
			})

			It("logs the correct btrfs command", func() {
				volumeID := randVolumeID()
				destVolID := randVolumeID()

				fromPath, err := driver.CreateVolume(logger, "", volumeID)
				Expect(err).NotTo(HaveOccurred())

				destVolPath, err := driver.CreateVolume(logger, volumeID, destVolID)
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
				volPath := filepath.Join(storePath, store.VolumesDirName, volID)
				Expect(os.MkdirAll(volPath, 0777)).To(Succeed())

				_, err := driver.CreateVolume(logger, "", volID)
				Expect(err).To(MatchError(ContainSubstring("creating btrfs volume")))
			})
		})

		Context("when the parent does not exist", func() {
			It("returns an error", func() {
				volID := randVolumeID()

				_, err := driver.CreateVolume(logger, "non-existent-parent", volID)
				Expect(err).To(MatchError(ContainSubstring("creating btrfs volume")))
			})
		})
	})

	Describe("CreateImage", func() {
		var (
			spec     image_cloner.ImageDriverSpec
			volumeID string
		)

		BeforeEach(func() {
			driver = btrfs.NewDriver("btrfs", "mkfs.btrfs", draxBinPath, storePath)
			volumeID = randVolumeID()
			volumePath, err := driver.CreateVolume(logger, "", volumeID)
			Expect(err).NotTo(HaveOccurred())
			Expect(ioutil.WriteFile(filepath.Join(volumePath, "a_file"), []byte("hello-world"), 0666)).To(Succeed())

			imagePath, err := ioutil.TempDir(storePath, "")
			Expect(err).NotTo(HaveOccurred())
			spec = image_cloner.ImageDriverSpec{
				ImagePath:     imagePath,
				BaseVolumeIDs: []string{volumeID},
				Mount:         true,
			}
		})

		It("creates a btrfs snapshot", func() {
			_, err := driver.CreateImage(logger, spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(filepath.Join(spec.ImagePath, "rootfs", "a_file")).To(BeARegularFile())
		})

		It("logs the correct btrfs command", func() {
			_, err := driver.CreateImage(logger, spec)
			Expect(err).NotTo(HaveOccurred())

			Expect(logger).To(ContainSequence(
				Debug(
					Message("btrfs.btrfs-creating-snapshot.starting-btrfs"),
					Data("args", []string{"btrfs", "subvolume", "snapshot", filepath.Join(storePath, store.VolumesDirName, volumeID), filepath.Join(spec.ImagePath, "rootfs")}),
					Data("path", "/bin/btrfs"),
				),
			))
		})

		It("doesn't apply any quota", func() {
			spec.DiskLimit = 0
			_, err := driver.CreateImage(logger, spec)
			Expect(err).NotTo(HaveOccurred())

			Expect(logger).To(ContainSequence(
				Debug(
					Message("btrfs.btrfs-creating-snapshot.applying-quotas.no-need-for-quotas"),
				),
			))
		})

		It("returns an empty mountJson object", func() {
			mountJson, err := driver.CreateImage(logger, spec)
			Expect(err).ToNot(HaveOccurred())
			Expect(mountJson).To(Equal(groot.MountInfo{}))
		})

		Context("when mount is false", func() {
			BeforeEach(func() {
				spec.Mount = false
			})

			It("keeps the rootfs path empty", func() {
				_, err := driver.CreateImage(logger, spec)
				Expect(err).ToNot(HaveOccurred())

				contents, err := ioutil.ReadDir(filepath.Join(spec.ImagePath, "rootfs"))
				Expect(err).NotTo(HaveOccurred())
				Expect(contents).To(BeEmpty())
			})

			It("creates a snapshot folder with volume contents", func() {
				_, err := driver.CreateImage(logger, spec)
				Expect(err).NotTo(HaveOccurred())
				Expect(filepath.Join(spec.ImagePath, "snapshot", "a_file")).To(BeARegularFile())
			})

			It("returns the correct mount information", func() {
				mountInfo, err := driver.CreateImage(logger, spec)
				Expect(err).NotTo(HaveOccurred())

				Expect(mountInfo.Type).To(Equal(""))
				Expect(mountInfo.Destination).To(Equal(filepath.Join(spec.ImagePath, "rootfs")))
				Expect(mountInfo.Source).To(Equal(filepath.Join(spec.ImagePath, "snapshot")))
				Expect(mountInfo.Options).To(HaveLen(1))
				Expect(mountInfo.Options[0]).To(Equal("bind"))
			})
		})

		Context("when disk limit is > 0", func() {
			var snapshotPath string

			BeforeEach(func() {
				spec.DiskLimit = 1024 * 1024 * 10
				snapshotPath = filepath.Join(spec.ImagePath, "rootfs")
			})

			Context("when the snapshot path is a volume", func() {
				JustBeforeEach(func() {
					cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(storePath, store.VolumesDirName, volumeID, "vol-file")), "bs=1048576", "count=5")
					sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))
				})

				It("applies the disk limit", func() {
					_, err := driver.CreateImage(logger, spec)
					Expect(err).NotTo(HaveOccurred())

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
						Expect(os.MkdirAll(filepath.Join(spec.ImagePath, "rootfs"), 0755)).To(Succeed())
					})

					AfterEach(func() {
						Expect(os.RemoveAll(tempFolder)).To(Succeed())
					})

					It("will force drax to use that binary", func() {
						driver = btrfs.NewDriver(btrfsBin.Name(), "mkfs.btrfs", draxBinPath, storePath)
						_, err := driver.CreateImage(logger, spec)
						Expect(err).NotTo(HaveOccurred())

						contents, err := ioutil.ReadFile(btrfsCalledFile.Name())
						Expect(err).NotTo(HaveOccurred())
						Expect(string(contents)).To(Equal("I'm groot - btrfs"))
					})
				})

				Context("when exclusive limit is active", func() {
					BeforeEach(func() {
						spec.ExclusiveDiskLimit = true
					})

					It("applies the disk limit with the exclusive flag", func() {
						_, err := driver.CreateImage(logger, spec)
						Expect(err).NotTo(HaveOccurred())

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
						_, err := driver.CreateImage(logger, spec)
						Expect(err).To(MatchError(ContainSubstring("drax was not found in the $PATH")))
					})
				})
			})
		})

		Context("custom btrfs binary path", func() {
			It("uses the custom btrfs binary given", func() {
				driver = btrfs.NewDriver("cool-btrfs", "mkfs.btrfs", draxBinPath, storePath)
				_, err := driver.CreateImage(logger, spec)
				Expect(err).To(MatchError(ContainSubstring(`"cool-btrfs": executable file not found in $PATH`)))
			})
		})

		Context("when the parent does not exist", func() {
			It("returns an error", func() {
				spec.BaseVolumeIDs = []string{"non-existent-parent"}
				_, err := driver.CreateImage(logger, spec)
				Expect(err).To(MatchError(ContainSubstring("creating btrfs snapshot")))
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
			It("returns an error", func() {
				driver := btrfs.NewDriver("btrfs", "mkfs.btrfs", draxBinPath, storePath)
				Expect(os.RemoveAll(filepath.Join(storePath, store.VolumesDirName))).To(Succeed())
				_, err := driver.Volumes(logger)
				Expect(err).To(MatchError(ContainSubstring("failed to list volumes")))
			})
		})
	})

	Describe("MoveVolume", func() {
		var (
			volumeID   string
			volumePath string
		)

		JustBeforeEach(func() {
			volumeID = randVolumeID()
			var err error
			volumePath, err = driver.CreateVolume(logger, "", volumeID)
			Expect(err).NotTo(HaveOccurred())
		})

		It("moves the volume to the given location", func() {
			newVolumePath := fmt.Sprintf("%s-new", volumePath)

			_, err := os.Stat(newVolumePath)
			Expect(err).To(HaveOccurred())
			Expect(os.IsNotExist(err)).To(BeTrue())

			err = driver.MoveVolume(logger, volumePath, newVolumePath)
			Expect(err).ToNot(HaveOccurred())
			stat, err := os.Stat(newVolumePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(stat.IsDir()).To(BeTrue())
		})

		Context("when the source volume does not exist", func() {
			It("returns an error", func() {
				newVolumePath := fmt.Sprintf("%s-new", volumePath)
				err := driver.MoveVolume(logger, "nonsense", newVolumePath)
				Expect(err).To(MatchError(ContainSubstring("moving volume")))
			})
		})

		Context("when the target volume already exists", func() {
			It("returns without error", func() {
				err := driver.MoveVolume(logger, volumePath, filepath.Dir(volumePath))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("HandleOpaqueWhiteouts", func() {
		var (
			opaqueWhiteouts []string
			volumeID        string
			volumePath      string
		)

		JustBeforeEach(func() {
			volumeID = randVolumeID()
			var err error
			volumePath, err = driver.CreateVolume(logger, "", volumeID)
			Expect(err).NotTo(HaveOccurred())

			Expect(os.MkdirAll(filepath.Join(volumePath, "a/b"), 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(volumePath, "a/b/file_1"), []byte{}, 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(volumePath, "a/b/file_2"), []byte{}, 0755)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(volumePath, "c/d/e"), 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(volumePath, "c/d/file_1"), []byte{}, 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(volumePath, "c/d/file_2"), []byte{}, 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(volumePath, "c/d/e/file_3"), []byte{}, 0755)).To(Succeed())

			opaqueWhiteouts = []string{
				"/a/b/.opaque",
				"c/d/.opaque",
			}
		})

		It("empties the given folders within a volume", func() {
			Expect(driver.HandleOpaqueWhiteouts(logger, volumeID, opaqueWhiteouts)).To(Succeed())

			abFolderPath := filepath.Join(volumePath, "a/b")
			Expect(abFolderPath).To(BeADirectory())
			files, err := ioutil.ReadDir(abFolderPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(BeEmpty())

			cdFolderPath := filepath.Join(volumePath, "c/d")
			Expect(cdFolderPath).To(BeADirectory())
			files, err = ioutil.ReadDir(cdFolderPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(BeEmpty())
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
			volumePath, err = driver.CreateVolume(logger, "", volumeID)
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes the btrfs volume by id", func() {
			Expect(volumePath).To(BeADirectory())

			Expect(driver.DestroyVolume(logger, volumeID)).To(Succeed())
			Expect(volumePath).ToNot(BeAnExistingFile())
		})

		It("deletes the metadata file", func() {
			metaFilePath := filepath.Join(storePath, store.MetaDirName, fmt.Sprintf("volume-%s", volumeID))
			Expect(ioutil.WriteFile(metaFilePath, []byte{}, 0644)).To(Succeed())
			Expect(driver.DestroyVolume(logger, volumeID)).To(Succeed())
			Expect(metaFilePath).ToNot(BeAnExistingFile())
		})

		Context("custom btrfs binary path", func() {
			It("uses the custom btrfs binary given", func() {
				driver = btrfs.NewDriver("cool-btrfs", "mkfs.btrfs", draxBinPath, storePath)
				err := driver.DestroyVolume(logger, volumeID)
				Expect(err).To(MatchError(ContainSubstring(`"cool-btrfs": executable file not found in $PATH`)))
			})
		})
	})

	Describe("DestroyImage", func() {
		var (
			spec       image_cloner.ImageDriverSpec
			rootfsPath string
		)

		Context("when a volume exists", func() {
			BeforeEach(func() {
				spec = image_cloner.ImageDriverSpec{
					Mount: true,
				}
			})

			JustBeforeEach(func() {
				volumeID := randVolumeID()
				_, err := driver.CreateVolume(logger, "", volumeID)
				Expect(err).NotTo(HaveOccurred())

				imagePath := filepath.Join(storePath, store.ImageDirName, "image-id")
				Expect(os.MkdirAll(imagePath, 0777)).To(Succeed())

				spec.ImagePath = imagePath
				spec.BaseVolumeIDs = []string{volumeID}

				_, err = driver.CreateImage(logger, spec)
				Expect(err).NotTo(HaveOccurred())
				rootfsPath = filepath.Join(imagePath, "rootfs")
			})

			It("deletes the btrfs volume", func() {
				Expect(rootfsPath).To(BeADirectory())

				Expect(driver.DestroyImage(logger, spec.ImagePath)).To(Succeed())
				Expect(rootfsPath).ToNot(BeAnExistingFile())
			})

			It("deletes the quota group for the volume", func() {
				rootIDBuffer := gbytes.NewBuffer()
				sess, err := gexec.Start(exec.Command("sudo", "btrfs", "inspect-internal", "rootid", rootfsPath), rootIDBuffer, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))
				rootID := strings.TrimSpace(string(rootIDBuffer.Contents()))

				sess, err = gexec.Start(exec.Command("sudo", "btrfs", "qgroup", "show", storePath), GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess).To(gbytes.Say(rootID))

				Expect(driver.DestroyImage(logger, spec.ImagePath)).To(Succeed())

				sess, err = gexec.Start(exec.Command("sudo", "btrfs", "qgroup", "show", storePath), GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess).ToNot(gbytes.Say(rootID))
			})

			It("logs the correct btrfs command", func() {
				Expect(rootfsPath).To(BeADirectory())

				Expect(driver.DestroyImage(logger, spec.ImagePath)).To(Succeed())

				Expect(logger).To(ContainSequence(
					Debug(
						Message("btrfs.btrfs-destroying-image.destroying-subvolume.starting-btrfs"),
						Data("path", "/bin/btrfs"),
						Data("args", []string{"btrfs", "subvolume", "delete", rootfsPath}),
					),
				))
			})

			Context("when the rootfs folder has subvolumes inside", func() {
				JustBeforeEach(func() {
					sess, err := gexec.Start(exec.Command("btrfs", "sub", "create", filepath.Join(rootfsPath, "subvolume")), GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))

					sess, err = gexec.Start(exec.Command("btrfs", "sub", "create", filepath.Join(rootfsPath, "subvolume", "subsubvolume")), GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))

					sess, err = gexec.Start(exec.Command("btrfs", "sub", "snapshot", filepath.Join(rootfsPath, "subvolume"), filepath.Join(rootfsPath, "snapshot")), GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))
				})

				It("deletes the btrfs volume", func() {
					Expect(rootfsPath).To(BeADirectory())
					Expect(driver.DestroyImage(logger, spec.ImagePath)).To(Succeed())
					Expect(rootfsPath).ToNot(BeAnExistingFile())
				})
			})

			Context("when the image was created with mount set to false", func() {
				BeforeEach(func() {
					spec.Mount = false
				})

				It("removes the image rootfs and snapshot folders", func() {
					snapshotPath := filepath.Join(spec.ImagePath, "snapshot")
					Expect(snapshotPath).To(BeADirectory())
					Expect(rootfsPath).To(BeADirectory())

					Expect(driver.DestroyImage(logger, spec.ImagePath)).To(Succeed())
					Expect(snapshotPath).ToNot(BeAnExistingFile())
					Expect(rootfsPath).ToNot(BeAnExistingFile())
				})

				Context("when the rootfs folder is not empty", func() {
					It("returns an error", func() {
						Expect(ioutil.WriteFile(filepath.Join(rootfsPath, "file"), []byte{}, 0700)).To(Succeed())
						err := driver.DestroyImage(logger, spec.ImagePath)
						Expect(err).To(MatchError(ContainSubstring("remove rootfs folder")))
					})
				})
			})

			Context("custom btrfs binary path", func() {
				It("uses the custom btrfs binary given", func() {
					driver = btrfs.NewDriver("cool-btrfs", "mkfs.btrfs", draxBinPath, storePath)
					err := driver.DestroyImage(logger, spec.ImagePath)
					Expect(err).To(MatchError(ContainSubstring(`"cool-btrfs": executable file not found in $PATH`)))
				})
			})

			Context("and drax does not exist", func() {
				BeforeEach(func() {
					draxBinPath = "/path/to/non-existent-drax"
				})

				It("succeeds", func() {
					Expect(driver.DestroyImage(logger, spec.ImagePath)).To(Succeed())
				})
			})
		})

		Context("when destroying a non existant volume", func() {
			It("returns an error", func() {
				err := driver.DestroyImage(logger, "non-existant")

				Expect(err).To(MatchError(ContainSubstring("image path not found")))
			})
		})

		Context("when destroying the volume fails", func() {
			It("returns an error", func() {
				tmpDir, _ := ioutil.TempDir("", "")
				Expect(os.Mkdir(filepath.Join(tmpDir, "rootfs"), 0755)).To(Succeed())

				err := driver.DestroyImage(logger, tmpDir)
				Expect(err).To(MatchError(ContainSubstring("destroying volume")))
			})
		})
	})

	Describe("FetchStats", func() {
		var (
			toPath    string
			imagePath string
		)

		BeforeEach(func() {
			var err error
			imagePath, err = ioutil.TempDir(storePath, "")
			Expect(err).NotTo(HaveOccurred())
			toPath = filepath.Join(imagePath, "rootfs")
		})

		Context("when the provided path is a volume", func() {
			JustBeforeEach(func() {
				volID := randVolumeID()
				volPath, err := driver.CreateVolume(logger, "", volID)
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(volPath, "vol-file")), "bs=4210688", "count=1")
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				spec := image_cloner.ImageDriverSpec{
					ImagePath:     imagePath,
					BaseVolumeIDs: []string{volID},
					Mount:         true,
				}

				_, err = driver.CreateImage(logger, spec)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the correct stats", func() {
				cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(toPath, "hello")), "bs=4210688", "count=1")
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				stats, err := driver.FetchStats(logger, imagePath)
				Expect(err).ToNot(HaveOccurred())

				// Block math craziness -> 1* 4210688 ~= 4214784
				Expect(stats.DiskUsage.ExclusiveBytesUsed).To(BeNumerically("~", 4214784, 100))
				// Block math craziness -> 2* 4227072 ~= 8425472
				Expect(stats.DiskUsage.TotalBytesUsed).To(BeNumerically("~", 8425472, 100))
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
					driver = btrfs.NewDriver(btrfsBin.Name(), "mkfs.btrfs", draxBinPath, storePath)
					_, err := driver.FetchStats(logger, imagePath)
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
					_, err := driver.FetchStats(logger, imagePath)
					Expect(err).To(MatchError(ContainSubstring("drax was not found in the $PATH")))
				})
			})
		})

		Context("when the provided path is not a volume", func() {
			BeforeEach(func() {
				Expect(os.MkdirAll(toPath, 0777)).To(Succeed())
			})

			It("returns an error", func() {
				_, err := driver.FetchStats(logger, imagePath)
				Expect(err).To(MatchError(ContainSubstring("is not a btrfs volume")))
			})
		})

		Context("when path does not exist", func() {
			BeforeEach(func() {
				toPath = "/tmp/not-here"
			})

			It("returns an error", func() {
				_, err := driver.FetchStats(logger, imagePath)
				Expect(err).To(MatchError(ContainSubstring("No such file or directory")))
			})
		})
	})

	Describe("WriteVolumeMeta", func() {
		It("creates the correct metadata file", func() {
			err := driver.WriteVolumeMeta(logger, "1234", base_image_puller.VolumeMeta{Size: 1024})
			Expect(err).NotTo(HaveOccurred())

			metaFilePath := filepath.Join(storePath, store.MetaDirName, "volume-1234")
			Expect(metaFilePath).To(BeAnExistingFile())
			metaFile, err := os.Open(metaFilePath)
			Expect(err).NotTo(HaveOccurred())
			var meta base_image_puller.VolumeMeta

			Expect(json.NewDecoder(metaFile).Decode(&meta)).To(Succeed())
			Expect(meta).To(Equal(base_image_puller.VolumeMeta{Size: 1024}))
		})
	})

	Describe("VolumeSize", func() {
		It("returns the volume size", func() {
			volumeID := randVolumeID()
			createVolume(storePath, driver, "", volumeID, 3000000)
			size, err := driver.VolumeSize(logger, volumeID)
			Expect(err).NotTo(HaveOccurred())
			Expect(size).To(BeEquivalentTo(3000000))
		})
	})
})

func randVolumeID() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("volume-%d", r.Int())
}

func createVolume(storePath string, driver *btrfs.Driver, parentID, id string, size int64) string {
	path, err := driver.CreateVolume(lagertest.NewTestLogger("test"), parentID, id)
	Expect(err).NotTo(HaveOccurred())
	metaFilePath := filepath.Join(storePath, store.MetaDirName, fmt.Sprintf("volume-%s", id))
	metaContents := fmt.Sprintf(`{"Size": %d}`, size)
	Expect(ioutil.WriteFile(metaFilePath, []byte(metaContents), 0644)).To(Succeed())

	return path
}

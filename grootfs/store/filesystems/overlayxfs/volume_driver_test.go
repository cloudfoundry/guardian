package overlayxfs_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VolumeDriver", func() {
	var (
		err      error
		driver   *overlayxfs.Driver
		logger   *lagertest.TestLogger
		randomID string
	)

	BeforeEach(func() {
		randomID = randVolumeID()
		Expect(os.MkdirAll(filepath.Join(StorePath, store.VolumesDirName), 0777)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(StorePath, store.ImageDirName), 0777)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(StorePath, store.LinksDirName), 0777)).To(Succeed())

		driver, err = overlayxfs.NewDriver(StorePath)
		Expect(err).NotTo(HaveOccurred())
		logger = lagertest.NewTestLogger("overlay+xfs")
	})

	AfterEach(func() {
		os.RemoveAll(filepath.Join(StorePath, store.VolumesDirName))
		os.RemoveAll(filepath.Join(StorePath, store.ImageDirName))
		os.RemoveAll(filepath.Join(StorePath, store.LinksDirName))
	})

	Context("when the storePath is not a xfs volume", func() {
		It("returns an error", func() {
			_, err = overlayxfs.NewDriver("/mnt/ext4")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("filesystem driver requires store filesystem to be xfs")))
		})
	})

	Describe("VolumePath", func() {
		BeforeEach(func() {
			Expect(os.MkdirAll(filepath.Join(StorePath, store.VolumesDirName, randomID), 0755)).To(Succeed())
		})

		It("returns the volume path when it exists", func() {
			retVolPath, err := driver.VolumePath(logger, randomID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retVolPath).To(Equal(filepath.Join(StorePath, store.VolumesDirName, randomID)))
		})

		Context("when the volume does not exist", func() {
			It("returns an error", func() {
				_, err := driver.VolumePath(logger, "non-existent-id")
				Expect(err).To(MatchError(ContainSubstring("volume does not exist")))
			})
		})
	})

	Describe("Create", func() {
		It("creates a volume", func() {
			expectedVolumePath := filepath.Join(StorePath, store.VolumesDirName, randomID)
			Expect(expectedVolumePath).NotTo(BeAnExistingFile())

			volumePath, err := driver.CreateVolume(logger, "parent-id", randomID)
			Expect(err).NotTo(HaveOccurred())

			Expect(expectedVolumePath).To(BeADirectory())
			Expect(volumePath).To(Equal(expectedVolumePath))

			linkFile := filepath.Join(StorePath, store.LinksDirName, randomID)
			_, err = os.Stat(linkFile)
			Expect(err).ToNot(HaveOccurred(), "volume link file has not been created")

			linkName, err := ioutil.ReadFile(linkFile)
			Expect(err).ToNot(HaveOccurred(), "failed to read volume link file")

			link := filepath.Join(StorePath, store.LinksDirName, string(linkName))
			linkStat, err := os.Lstat(link)
			Expect(err).ToNot(HaveOccurred())
			Expect(linkStat.Mode()&os.ModeSymlink).ToNot(
				BeZero(),
				fmt.Sprintf("Volume link %s is not a symlink", link),
			)
			Expect(os.Readlink(link)).To(Equal(volumePath), "Volume link does not point to volume")
		})

		Context("when volume dir doesn't exist", func() {
			BeforeEach(func() {
				Expect(os.Remove(filepath.Join(StorePath, store.VolumesDirName))).To(Succeed())
			})

			It("returns an error", func() {
				_, err := driver.CreateVolume(logger, "parent-id", randomID)
				Expect(err).To(MatchError(ContainSubstring("creating volume")))
			})
		})

		Context("when volume already exists", func() {
			BeforeEach(func() {
				Expect(os.Mkdir(filepath.Join(StorePath, store.VolumesDirName, randomID), 0755)).To(Succeed())
			})

			It("returns an error", func() {
				_, err := driver.CreateVolume(logger, "parent-id", randomID)
				Expect(err).To(MatchError(ContainSubstring("creating volume")))
			})
		})
	})

	Describe("Volumes", func() {
		var volumesPath string
		BeforeEach(func() {
			volumesPath = filepath.Join(StorePath, store.VolumesDirName)
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
				driver, err := overlayxfs.NewDriver(StorePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(os.RemoveAll(filepath.Join(StorePath, store.VolumesDirName))).To(Succeed())
				_, err = driver.Volumes(logger)
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
			volumePath, err = driver.CreateVolume(logger, "", volumeID)
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes the overlayxfs volume by id", func() {
			Expect(volumePath).To(BeADirectory())

			Expect(driver.DestroyVolume(logger, volumeID)).To(Succeed())
			Expect(volumePath).ToNot(BeAnExistingFile())
		})

		It("deletes the associated symlink", func() {
			Expect(volumePath).To(BeADirectory())
			linkFilePath := filepath.Join(StorePath, store.LinksDirName, volumeID)
			Expect(linkFilePath).To(BeAnExistingFile())
			linkfileinfo, err := ioutil.ReadFile(linkFilePath)
			symlinkPath := filepath.Join(StorePath, store.LinksDirName, string(linkfileinfo))
			Expect(err).ToNot(HaveOccurred())
			Expect(symlinkPath).To(BeAnExistingFile())

			Expect(driver.DestroyVolume(logger, volumeID)).To(Succeed())
			Expect(volumePath).ToNot(BeAnExistingFile())
			Expect(linkFilePath).ToNot(BeAnExistingFile())
			Expect(symlinkPath).ToNot(BeAnExistingFile())
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

			stat, err := os.Stat(newVolumePath)
			Expect(err).To(HaveOccurred())
			Expect(os.IsNotExist(err)).To(BeTrue())

			err = driver.MoveVolume(logger, volumePath, newVolumePath)
			Expect(err).ToNot(HaveOccurred())
			stat, err = os.Stat(newVolumePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(stat.IsDir()).To(BeTrue())
		})

		It("updates the volume link to point to the new volume location", func() {
			newVolumePath := fmt.Sprintf("%s-new", volumePath)
			_, err := os.Stat(newVolumePath)
			Expect(err).To(HaveOccurred())
			Expect(os.IsNotExist(err)).To(BeTrue())
			fileInVolume := "file-in-volume"
			filePath := filepath.Join(volumePath, fileInVolume)
			f, err := os.Create(filePath)
			Expect(err).ToNot(HaveOccurred())
			f.Close()

			err = driver.MoveVolume(logger, volumePath, newVolumePath)
			Expect(err).ToNot(HaveOccurred())

			linkName, err := ioutil.ReadFile(filepath.Join(StorePath, store.LinksDirName, filepath.Base(newVolumePath)))
			Expect(err).NotTo(HaveOccurred())
			linkPath := filepath.Join(StorePath, store.LinksDirName, string(linkName))
			_, err = os.Lstat(linkPath)
			Expect(err).NotTo(HaveOccurred())

			target, err := os.Readlink(linkPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(target).To(Equal(newVolumePath))

			_, err = os.Stat(filepath.Join(StorePath, store.LinksDirName, filepath.Base(newVolumePath)))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the source volume does not exist", func() {
			It("returns an error", func() {
				newVolumePath := fmt.Sprintf("%s-new", volumePath)
				err = driver.MoveVolume(logger, "nonsense", newVolumePath)
				Expect(err).To(MatchError(ContainSubstring("moving volume")))
			})
		})

		Context("when the target volume already exists", func() {
			It("returns an error", func() {
				err = driver.MoveVolume(logger, volumePath, filepath.Dir(volumePath))
				Expect(err).To(MatchError(ContainSubstring("moving volume")))
			})
		})
	})
})

func randVolumeID() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("volume-%d", r.Int())
}

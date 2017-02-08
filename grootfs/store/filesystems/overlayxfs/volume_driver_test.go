package overlayxfs_test

import (
	"fmt"
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
		driver   *overlayxfs.Driver
		logger   *lagertest.TestLogger
		randomID string
	)

	BeforeEach(func() {
		randomID = randVolumeID()
		Expect(os.MkdirAll(filepath.Join(StorePath, store.VolumesDirName), 0777)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(StorePath, store.ImageDirName), 0777)).To(Succeed())

		driver = overlayxfs.NewDriver(StorePath)
		logger = lagertest.NewTestLogger("overlay+xfs")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(filepath.Join(StorePath, store.VolumesDirName))).To(Succeed())
		Expect(os.RemoveAll(filepath.Join(StorePath, store.ImageDirName))).To(Succeed())
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
})

func randVolumeID() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("volume-%d", r.Int())
}

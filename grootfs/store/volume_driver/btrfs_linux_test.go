package volume_driver_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/volume_driver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/st3v/glager"
)

var _ = Describe("Btrfs", func() {
	const btrfsMountPath = "/mnt/btrfs"

	var (
		btrfs     *volume_driver.Btrfs
		logger    *TestLogger
		storePath string
	)

	BeforeEach(func() {
		storePath = filepath.Join(
			btrfsMountPath, fmt.Sprintf("test-store-%d", GinkgoParallelNode()),
		)
		volumesPath := filepath.Join(storePath, store.VOLUMES_DIR_NAME)
		Expect(os.MkdirAll(volumesPath, 0755)).To(Succeed())

		btrfs = volume_driver.NewBtrfs(storePath)

		logger = NewLogger("btrfs")
	})

	Describe("Path", func() {
		It("returns the volume path when it exists", func() {
			volID := randVolumeID()
			volPath, err := btrfs.Create(logger, "", volID)
			Expect(err).NotTo(HaveOccurred())

			retVolPath, err := btrfs.Path(logger, volID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retVolPath).To(Equal(volPath))
		})

		Context("when the volume does not exist", func() {
			It("returns an error", func() {
				_, err := btrfs.Path(logger, "non-existent-id")
				Expect(err).To(MatchError(ContainSubstring("volume does not exist")))
			})
		})
	})

	Describe("Create", func() {
		Context("when the parent is empty", func() {
			It("creates a BTRFS subvolume", func() {
				volID := randVolumeID()
				volPath, err := btrfs.Create(logger, "", volID)
				Expect(err).NotTo(HaveOccurred())

				Expect(volPath).To(BeADirectory())
			})

			It("logs the correct btrfs command", func() {
				volID := randVolumeID()
				volumePath, err := btrfs.Create(logger, "", volID)
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

		Context("when the parent is not empty", func() {
			It("creates a BTRFS snapshot", func() {
				volumeID := randVolumeID()
				destVolID := randVolumeID()

				fromPath, err := btrfs.Create(logger, "", volumeID)
				Expect(err).NotTo(HaveOccurred())

				Expect(ioutil.WriteFile(filepath.Join(fromPath, "a_file"), []byte("hello-world"), 0666)).To(Succeed())

				destVolPath, err := btrfs.Create(logger, volumeID, destVolID)
				Expect(err).NotTo(HaveOccurred())

				Expect(filepath.Join(destVolPath, "a_file")).To(BeARegularFile())
			})

			It("logs the correct btrfs command", func() {
				volumeID := randVolumeID()
				destVolID := randVolumeID()

				fromPath, err := btrfs.Create(logger, "", volumeID)
				Expect(err).NotTo(HaveOccurred())

				destVolPath, err := btrfs.Create(logger, volumeID, destVolID)
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

				_, err := btrfs.Create(logger, "", volID)
				Expect(err).To(MatchError(ContainSubstring("creating btrfs volume")))
			})
		})

		Context("when the parent does not exist", func() {
			It("returns an error", func() {
				volID := randVolumeID()

				_, err := btrfs.Create(logger, "non-existent-parent", volID)
				Expect(err).To(MatchError(ContainSubstring("creating btrfs volume")))
			})
		})
	})

	Describe("Snapshot", func() {
		var toPath string

		BeforeEach(func() {
			bundlePath, err := ioutil.TempDir(storePath, "")
			Expect(err).NotTo(HaveOccurred())
			toPath = filepath.Join(bundlePath, "rootfs")
		})

		It("creates a BTRFS snapshot", func() {
			volumeID := randVolumeID()

			fromPath, err := btrfs.Create(logger, "", volumeID)
			Expect(err).NotTo(HaveOccurred())

			Expect(ioutil.WriteFile(filepath.Join(fromPath, "a_file"), []byte("hello-world"), 0666)).To(Succeed())

			Expect(btrfs.Snapshot(logger, fromPath, toPath)).To(Succeed())

			Expect(filepath.Join(toPath, "a_file")).To(BeARegularFile())
		})

		It("logs the correct btrfs command", func() {
			volumeID := randVolumeID()

			fromPath, err := btrfs.Create(logger, "", volumeID)
			Expect(err).NotTo(HaveOccurred())

			Expect(ioutil.WriteFile(filepath.Join(fromPath, "a_file"), []byte("hello-world"), 0666)).To(Succeed())

			Expect(btrfs.Snapshot(logger, fromPath, toPath)).To(Succeed())

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

		Context("when the parent does not exist", func() {
			It("returns an error", func() {
				Expect(
					btrfs.Snapshot(logger, "non-existent-parent", toPath),
				).To(MatchError(ContainSubstring("creating btrfs snapshot")))
			})
		})
	})

	Describe("Destroy", func() {
		var volumePath string

		BeforeEach(func() {
			var err error
			volumePath, err = btrfs.Create(logger, "", randVolumeID())
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes the btrfs volume", func() {
			Expect(volumePath).To(BeADirectory())

			Expect(btrfs.Destroy(logger, volumePath)).To(Succeed())
			Expect(volumePath).ToNot(BeAnExistingFile())
		})

		It("logs the correct btrfs command", func() {
			Expect(volumePath).To(BeADirectory())

			Expect(btrfs.Destroy(logger, volumePath)).To(Succeed())

			Expect(logger).To(ContainSequence(
				Debug(
					Message("btrfs.btrfs-destroying.starting-btrfs"),
					Data("path", "/bin/btrfs"),
					Data("args", []string{"btrfs", "subvolume", "delete", volumePath}),
				),
			))
		})

		Context("when destroying a non existant volume", func() {
			It("returns an error", func() {
				err := btrfs.Destroy(logger, "non-existant")

				Expect(err).To(MatchError(ContainSubstring("bundle path not found")))
			})
		})

		Context("when destroying the volume fails", func() {
			It("returns an error", func() {
				tmpDir, _ := ioutil.TempDir("", "")
				err := btrfs.Destroy(logger, tmpDir)

				Expect(err).To(MatchError(ContainSubstring("destroying volume")))
			})
		})
	})
})

func randVolumeID() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("volume-%d", r.Int())
}

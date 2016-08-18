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
			volID := randVolID()
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
				volID := randVolID()
				volPath, err := btrfs.Create(logger, "", volID)
				Expect(err).NotTo(HaveOccurred())

				Expect(volPath).To(BeADirectory())
			})

			It("logs the correct btrfs command", func() {
				volID := randVolID()
				volumePath, err := btrfs.Create(logger, "", volID)
				Expect(err).NotTo(HaveOccurred())

				Expect(logger).To(ContainSequence(
					Debug(
						Message("btrfs.btrfs-creating-volume.starting-btrfs"),
						Data("path", "/sbin/btrfs"),
						Data("args", []string{"btrfs", "subvolume", "create", volumePath}),
						Data("id", volID),
					),
				))
			})
		})

		Context("when the parent is not empty", func() {
			It("creates a BTRFS snapshot", func() {
				srcVolID := randVolID()
				destVolID := randVolID()

				srcVolPath, err := btrfs.Create(logger, "", srcVolID)
				Expect(err).NotTo(HaveOccurred())

				Expect(ioutil.WriteFile(filepath.Join(srcVolPath, "a_file"), []byte("hello-world"), 0666)).To(Succeed())

				destVolPath, err := btrfs.Create(logger, srcVolID, destVolID)
				Expect(err).NotTo(HaveOccurred())

				Expect(filepath.Join(destVolPath, "a_file")).To(BeARegularFile())
			})

			It("logs the correct btrfs command", func() {
				srcVolID := randVolID()
				destVolID := randVolID()

				srcVolPath, err := btrfs.Create(logger, "", srcVolID)
				Expect(err).NotTo(HaveOccurred())

				destVolPath, err := btrfs.Create(logger, srcVolID, destVolID)
				Expect(err).NotTo(HaveOccurred())

				Expect(logger).To(ContainSequence(
					Debug(
						Message("btrfs.btrfs-creating-volume.starting-btrfs"),
						Data("path", "/sbin/btrfs"),
						Data("args", []string{"btrfs", "subvolume", "snapshot", srcVolPath, destVolPath}),
						Data("id", destVolID),
						Data("parentID", srcVolID),
					),
				))
			})
		})

		Context("when the volume exists", func() {
			It("returns an error", func() {
				volID := randVolID()
				volPath := filepath.Join(storePath, store.VOLUMES_DIR_NAME, volID)
				Expect(os.MkdirAll(volPath, 0777)).To(Succeed())

				_, err := btrfs.Create(logger, "", volID)
				Expect(err).To(MatchError(ContainSubstring("creating btrfs volume")))
			})
		})

		Context("when the parent does not exist", func() {
			It("returns an error", func() {
				volID := randVolID()

				_, err := btrfs.Create(logger, "non-existent-parent", volID)
				Expect(err).To(MatchError(ContainSubstring("creating btrfs volume")))
			})
		})
	})

	Describe("Snapshot", func() {
		var destPath string

		BeforeEach(func() {
			bundlePath, err := ioutil.TempDir(storePath, "")
			Expect(err).NotTo(HaveOccurred())
			destPath = filepath.Join(bundlePath, "rootfs")
		})

		It("creates a BTRFS snapshot", func() {
			srcVolID := randVolID()

			srcVolPath, err := btrfs.Create(logger, "", srcVolID)
			Expect(err).NotTo(HaveOccurred())

			Expect(ioutil.WriteFile(filepath.Join(srcVolPath, "a_file"), []byte("hello-world"), 0666)).To(Succeed())

			Expect(btrfs.Snapshot(logger, srcVolID, destPath)).To(Succeed())

			Expect(filepath.Join(destPath, "a_file")).To(BeARegularFile())
		})

		It("logs the correct btrfs command", func() {
			srcVolID := randVolID()

			srcVolPath, err := btrfs.Create(logger, "", srcVolID)
			Expect(err).NotTo(HaveOccurred())

			Expect(ioutil.WriteFile(filepath.Join(srcVolPath, "a_file"), []byte("hello-world"), 0666)).To(Succeed())

			Expect(btrfs.Snapshot(logger, srcVolID, destPath)).To(Succeed())

			Expect(logger).To(ContainSequence(
				Debug(
					Message("btrfs.btrfs-creating-snapshot.starting-btrfs"),
					Data("path", "/sbin/btrfs"),
					Data("args", []string{"btrfs", "subvolume", "snapshot", srcVolPath, destPath}),
					Data("id", srcVolID),
				),
			))
		})

		Context("when the parent does not exist", func() {
			It("returns an error", func() {
				Expect(
					btrfs.Snapshot(logger, "non-existent-parent", destPath),
				).To(MatchError(ContainSubstring("creating btrfs snapshot")))
			})
		})
	})
})

func randVolID() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("volume-%d", r.Int())
}

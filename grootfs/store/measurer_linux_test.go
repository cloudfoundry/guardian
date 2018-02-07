package store_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/storefakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Measurer", func() {
	var (
		storePath          string
		storeMeasurer      *store.StoreMeasurer
		logger             *lagertest.TestLogger
		volumeDriver       *storefakes.FakeVolumeDriver
		unusedVolumeGetter *storefakes.FakeUnusedVolumeGetter
	)

	BeforeEach(func() {
		mountPoint := fmt.Sprintf("/mnt/xfs-%d", GinkgoParallelNode())
		var err error
		storePath, err = ioutil.TempDir(mountPoint, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.MkdirAll(
			filepath.Join(storePath, store.VolumesDirName), 0744,
		)).To(Succeed())
		Expect(os.MkdirAll(
			filepath.Join(storePath, store.ImageDirName), 0744,
		)).To(Succeed())

		volumeDriver = new(storefakes.FakeVolumeDriver)
		unusedVolumeGetter = new(storefakes.FakeUnusedVolumeGetter)

		storeMeasurer = store.NewStoreMeasurer(storePath, volumeDriver, unusedVolumeGetter)

		logger = lagertest.NewTestLogger("store-measurer")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
	})

	Describe("Usage", func() {
		It("measures space used by the volumes and images", func() {
			volPath := filepath.Join(storePath, store.VolumesDirName, "sha256:fake")
			Expect(os.MkdirAll(volPath, 0744)).To(Succeed())
			Expect(writeFile(filepath.Join(volPath, "my-file"), 2048*1024)).To(Succeed())

			imagePath := filepath.Join(storePath, store.ImageDirName, "my-image")
			Expect(os.MkdirAll(imagePath, 0744)).To(Succeed())
			Expect(writeFile(filepath.Join(imagePath, "my-file"), 2048*1024)).To(Succeed())

			storeSize, err := storeMeasurer.Usage(logger)
			Expect(err).NotTo(HaveOccurred())
			xfsMetadataSize := 33755136
			Expect(storeSize).To(BeNumerically("~", 4096*1024+xfsMetadataSize, 256*1024))
		})

		Context("when the store does not exist", func() {
			BeforeEach(func() {
				storeMeasurer = store.NewStoreMeasurer("/path/to/non/existent/store", volumeDriver, unusedVolumeGetter)
			})

			It("returns a useful error", func() {
				_, err := storeMeasurer.Usage(logger)
				Expect(err).To(MatchError(ContainSubstring("/path/to/non/existent/store")))
			})
		})
	})

	Describe("TotalVolumesSize", func() {
		BeforeEach(func() {
			volumeDriver.VolumeSizeReturns(2048, nil)
			volumeDriver.VolumesReturns([]string{"sha256:fake1", "sha256:fake2"}, nil)
		})

		It("measures the size of all layers", func() {
			cacheUsage, err := storeMeasurer.TotalVolumesSize(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(cacheUsage).To(BeNumerically("==", 4096))
		})

		Context("when getting volumes returns an error", func() {
			BeforeEach(func() {
				volumeDriver.VolumesReturns([]string{}, errors.New("failed here"))
			})

			It("returns the error", func() {
				_, err := storeMeasurer.TotalVolumesSize(logger)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the driver VolumeSize returns an error", func() {
			BeforeEach(func() {
				volumeDriver.VolumeSizeReturns(0, errors.New("failed here"))
			})

			It("returns the error", func() {
				_, err := storeMeasurer.TotalVolumesSize(logger)
				Expect(err).To(HaveOccurred())
			})

			Context("but it's because a file doesn't exist", func() {
				BeforeEach(func() {
					volumeDriver.VolumeSizeReturns(0, os.ErrNotExist)
				})

				It("carries on silently", func() {
					_, err := storeMeasurer.TotalVolumesSize(logger)
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})
	})

	Describe("UnusedVolumeSize", func() {
		BeforeEach(func() {
			volumeDriver.VolumeSizeReturns(1024, nil)
			unusedVolumeGetter.UnusedVolumesReturns([]string{"sha256:fake1", "sha256:fake2"}, nil)
		})

		It("measures the size of the unused layers", func() {
			cacheUsage, err := storeMeasurer.UnusedVolumesSize(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(cacheUsage).To(BeNumerically("==", 2048))
		})

		Context("when getting volumes returns an error", func() {
			BeforeEach(func() {
				unusedVolumeGetter.UnusedVolumesReturns([]string{}, errors.New("failed here"))
			})

			It("returns the error", func() {
				_, err := storeMeasurer.UnusedVolumesSize(logger)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the driver VolumeSize returns an error", func() {
			BeforeEach(func() {
				volumeDriver.VolumeSizeReturns(0, errors.New("failed here"))
			})

			It("returns the error", func() {
				_, err := storeMeasurer.UnusedVolumesSize(logger)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("CommittedQuota", func() {
		BeforeEach(func() {
			image1Path := filepath.Join(storePath, store.ImageDirName, "my-image-1")
			Expect(os.MkdirAll(image1Path, 0744)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(image1Path, "image_quota"), []byte("1024"), 0777)).To(Succeed())

			image2Path := filepath.Join(storePath, store.ImageDirName, "my-image-2")
			Expect(os.MkdirAll(image2Path, 0744)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(image2Path, "image_quota"), []byte("2048"), 0777)).To(Succeed())
		})

		It("returns the committed size of the store", func() {
			committedSize, err := storeMeasurer.CommittedQuota(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(committedSize).To(BeNumerically("==", 3072))
		})

		It("ignores images without quota", func() {
			Expect(os.Remove(filepath.Join(storePath, store.ImageDirName, "my-image-2", "image_quota"))).To(Succeed())
			committedSize, err := storeMeasurer.CommittedQuota(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(committedSize).To(BeNumerically("==", 1024))
		})

		It("errors if it cannot read the images dir", func() {
			Expect(os.RemoveAll(filepath.Join(storePath, store.ImageDirName))).To(Succeed())
			_, err := storeMeasurer.CommittedQuota(logger)
			Expect(err).To(HaveOccurred())
		})

		It("errors when unable to read quota files containing garbage", func() {
			erroneousImagePath := filepath.Join(storePath, store.ImageDirName, "erroneous-image")
			Expect(os.MkdirAll(erroneousImagePath, 0744)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(erroneousImagePath, "image_quota"), []byte("what!?"), 0777)).To(Succeed())

			_, err := storeMeasurer.CommittedQuota(logger)
			Expect(err).To(HaveOccurred())
		})

		It("silently ignores empty quota files", func() {
			erroneousImagePath := filepath.Join(storePath, store.ImageDirName, "erroneous-image")
			Expect(os.MkdirAll(erroneousImagePath, 0744)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(erroneousImagePath, "image_quota"), []byte(""), 0777)).To(Succeed())

			_, err := storeMeasurer.CommittedQuota(logger)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

func writeFile(path string, size int64) error {
	cmd := exec.Command(
		"dd", "if=/dev/zero", fmt.Sprintf("of=%s", path),
		"bs=1024", fmt.Sprintf("count=%d", size/1024),
	)
	sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	if err != nil {
		return err
	}
	sess.Wait()

	out := sess.Buffer().Contents()
	exitCode := sess.ExitCode()
	if exitCode != 0 {
		return fmt.Errorf("du failed with exit code %d: %s", exitCode, string(out))
	}

	return nil
}

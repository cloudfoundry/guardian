package integration_test

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/store"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GenerateVolumeSizeMetadata", func() {
	const (
		rootUID = 0
		rootGID = 0
	)

	var (
		baseImageURL *url.URL
	)

	BeforeEach(func() {
		integration.SkipIfNonRoot(GrootfsTestUid)

		Runner = Runner.WithStore(StorePath)

		workDir, err := os.Getwd()
		Expect(err).NotTo(HaveOccurred())

		baseImageURL = integration.String2URL(fmt.Sprintf("oci:///%s/assets/oci-test-image/grootfs-busybox:latest", workDir))
		_, err = Runner.Create(groot.CreateSpec{
			ID:           "image",
			BaseImageURL: baseImageURL,
			Mount:        mountByDefault(),
		})
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		Expect(Runner.Delete("image")).To(Succeed())
		Expect(Runner.DeleteStore()).To(Succeed())
	})

	Context("when some volume metadata is missing", func() {
		var (
			deletedMetaPath   string
			deletedVolumeMeta base_image_puller.VolumeMeta
		)

		BeforeEach(func() {
			volumesMetaGlob := filepath.Join(StorePath, store.MetaDirName, "volume-*")
			volumeMetaFileNames, err := filepath.Glob(volumesMetaGlob)
			Expect(err).NotTo(HaveOccurred())

			Expect(volumeMetaFileNames).To(HaveLen(4))

			for _, volumeMetaFileName := range volumeMetaFileNames {
				deletedMetaPath = volumeMetaFileName
				metadataFile, err := os.Open(deletedMetaPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(json.NewDecoder(metadataFile).Decode(&deletedVolumeMeta)).To(Succeed())
				if deletedVolumeMeta.Size != 0 {
					break
				}
			}

			Expect(os.Remove(deletedMetaPath)).To(Succeed())
		})

		It("generates the missing metadata with the correct information", func() {
			volumeMetaFileNames, err := filepath.Glob(filepath.Join(StorePath, store.MetaDirName, "volume-*"))
			Expect(err).NotTo(HaveOccurred())
			Expect(volumeMetaFileNames).To(HaveLen(3))

			Expect(Runner.GenerateVolumeSizeMetadata()).To(Succeed())

			volumeMetaFileNames, err = filepath.Glob(filepath.Join(StorePath, store.MetaDirName, "volume-*"))
			Expect(err).NotTo(HaveOccurred())
			Expect(volumeMetaFileNames).To(HaveLen(4))

			var generatedVolumeMeta base_image_puller.VolumeMeta
			metadataFile, err := os.Open(deletedMetaPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(json.NewDecoder(metadataFile).Decode(&generatedVolumeMeta)).To(Succeed())
			Expect(generatedVolumeMeta.Size).To(BeNumerically("~", deletedVolumeMeta.Size, 13000))
		})

		It("allows create to run without error", func() {
			Expect(Runner.GenerateVolumeSizeMetadata()).To(Succeed())
			_, err := Runner.Create(groot.CreateSpec{
				ID:                        "image-1",
				BaseImageURL:              baseImageURL,
				Mount:                     mountByDefault(),
				DiskLimit:                 10000000,
				ExcludeBaseImageFromQuota: false,
			})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when no metadata is missing", func() {
		It("is a noop", func() {
			volumeMetaFileNames, err := filepath.Glob(filepath.Join(StorePath, store.MetaDirName, "volume-*"))
			Expect(err).NotTo(HaveOccurred())
			Expect(volumeMetaFileNames).To(HaveLen(4))

			Expect(Runner.GenerateVolumeSizeMetadata()).To(Succeed())

			volumeMetaFileNames, err = filepath.Glob(filepath.Join(StorePath, store.MetaDirName, "volume-*"))
			Expect(err).NotTo(HaveOccurred())
			Expect(volumeMetaFileNames).To(HaveLen(4))
		})
	})

})

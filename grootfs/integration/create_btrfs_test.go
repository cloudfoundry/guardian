package integration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Create (btrfs only)", func() {
	var (
		baseImagePath   string
		sourceImagePath string
	)

	BeforeEach(func() {
		integration.SkipIfNotBTRFS(Driver)

		var err error
		sourceImagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		Expect(ioutil.WriteFile(path.Join(sourceImagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(sourceImagePath)).To(Succeed())
		Expect(os.RemoveAll(baseImagePath)).To(Succeed())
	})

	JustBeforeEach(func() {
		baseImageFile := integration.CreateBaseImageTar(sourceImagePath)
		baseImagePath = baseImageFile.Name()
	})

	Context("when inclusive disk limit is provided", func() {
		BeforeEach(func() {
			Expect(writeMegabytes(filepath.Join(sourceImagePath, "fatfile"), 5)).To(Succeed())
		})

		Describe("--btrfs-bin global flag", func() {
			var (
				btrfsCalledFile *os.File
				btrfsBin        *os.File
				tempFolder      string
			)

			BeforeEach(func() {
				tempFolder, btrfsBin, btrfsCalledFile = integration.CreateFakeBin("btrfs")
			})

			Context("when it's provided", func() {
				It("uses calls the provided btrfs binary", func() {
					_, err := Runner.WithBtrfsBin(btrfsBin.Name()).Create(groot.CreateSpec{BaseImage: baseImagePath, ID: "random-id"})
					Expect(err).To(HaveOccurred())

					contents, err := ioutil.ReadFile(btrfsCalledFile.Name())
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("I'm groot - btrfs"))
				})
			})

			Context("when it's not provided", func() {
				It("uses btrfs from $PATH", func() {
					newPATH := fmt.Sprintf("%s:%s", tempFolder, os.Getenv("PATH"))
					_, err := Runner.WithEnvVar(fmt.Sprintf("PATH=%s", newPATH)).Create(groot.CreateSpec{
						BaseImage: baseImagePath,
						ID:        "random-id",
						DiskLimit: tenMegabytes,
					})
					Expect(err).To(HaveOccurred())

					contents, err := ioutil.ReadFile(btrfsCalledFile.Name())
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("I'm groot - btrfs"))
				})
			})
		})
	})

	Describe("--config global flag", func() {
		Describe("btrfs bin", func() {
			var (
				btrfsCalledFile *os.File
				btrfsBin        *os.File
				tempFolder      string
			)

			BeforeEach(func() {
				tempFolder, btrfsBin, btrfsCalledFile = integration.CreateFakeBin("btrfs")
				cfg := config.Config{BtrfsBin: btrfsBin.Name()}
				Expect(Runner.SetConfig(cfg)).To(Succeed())
			})

			It("uses the btrfs bin from the config file", func() {
				_, err := Runner.Create(groot.CreateSpec{
					BaseImage: baseImagePath,
					ID:        "random-id",
				})
				Expect(err).To(HaveOccurred())

				contents, err := ioutil.ReadFile(btrfsCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - btrfs"))
			})
		})
	})

	Describe("clean up on create", func() {
		var (
			imageID string
		)

		BeforeEach(func() {
			integration.SkipIfNotBTRFS(Driver)
		})

		JustBeforeEach(func() {
			_, err := Runner.Create(groot.CreateSpec{
				ID:        "my-busybox",
				BaseImage: "docker:///busybox:1.26.2",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(Runner.Delete("my-busybox")).To(Succeed())
			imageID = "random-id"
		})

		AfterEach(func() {
			Expect(Runner.Delete(imageID)).To(Succeed())
		})

		It("cleans up unused layers before create but not the one about to be created", func() {
			runner := Runner.WithClean()

			createSpec := groot.CreateSpec{
				ID:        "my-empty",
				BaseImage: "docker:///cfgarden/empty:v0.1.1",
			}
			_, err := Runner.Create(createSpec)
			Expect(err).NotTo(HaveOccurred())
			Expect(runner.Delete("my-empty")).To(Succeed())

			layerPath := filepath.Join(StorePath, store.VolumesDirName, testhelpers.EmptyBaseImageV011.Layers[0].ChainID)
			stat, err := os.Stat(layerPath)
			Expect(err).NotTo(HaveOccurred())
			preLayerTimestamp := stat.ModTime()

			preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
			Expect(err).NotTo(HaveOccurred())
			Expect(preContents).To(HaveLen(3))

			_, err = runner.Create(groot.CreateSpec{
				ID:        imageID,
				BaseImage: "docker:///cfgarden/empty:v0.1.1",
			})
			Expect(err).NotTo(HaveOccurred())

			afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
			Expect(err).NotTo(HaveOccurred())
			Expect(afterContents).To(HaveLen(2))

			for _, layer := range testhelpers.EmptyBaseImageV011.Layers {
				Expect(filepath.Join(StorePath, store.VolumesDirName, layer.ChainID)).To(BeADirectory())
			}

			stat, err = os.Stat(layerPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.ModTime()).To(Equal(preLayerTimestamp))
		})

		Context("when no-clean flag is set", func() {
			It("does not clean up unused layers", func() {
				preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
				Expect(err).NotTo(HaveOccurred())
				Expect(preContents).To(HaveLen(1))

				_, err = Runner.WithNoClean().Create(groot.CreateSpec{
					ID:        imageID,
					BaseImage: "docker:///cfgarden/empty:v0.1.1",
				})
				Expect(err).NotTo(HaveOccurred())

				afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
				Expect(err).NotTo(HaveOccurred())
				Expect(afterContents).To(HaveLen(3))
			})
		})
	})
})

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
	"code.cloudfoundry.org/grootfs/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Create (btrfs only)", func() {
	var (
		baseImagePath   string
		sourceImagePath string
		spec            groot.CreateSpec
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

		spec = groot.CreateSpec{
			BaseImage: baseImagePath,
			ID:        "random-id",
			Mount:     true,
			DiskLimit: tenMegabytes,
		}
	})

	Describe("--drax-bin global flag", func() {
		var (
			draxCalledFile *os.File
			draxBin        *os.File
			tempFolder     string
		)

		BeforeEach(func() {
			tempFolder, draxBin, draxCalledFile = integration.CreateFakeDrax()
		})

		Context("when it's provided", func() {
			It("uses the provided drax", func() {
				_, err := Runner.WithDraxBin(draxBin.Name()).Create(spec)
				Expect(err).NotTo(HaveOccurred())

				contents, err := ioutil.ReadFile(draxCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - drax"))
			})

			Context("when the drax bin doesn't have uid bit set", func() {
				It("doesn't leak the image dir", func() {
					testhelpers.UnsuidBinary(draxBin.Name())
					_, err := Runner.WithDraxBin(draxBin.Name()).Create(spec)
					Expect(err).To(HaveOccurred())

					imagePath := path.Join(Runner.StorePath, "images", "random-id")
					Expect(imagePath).ToNot(BeAnExistingFile())
				})
			})
		})

		Context("when it's not provided", func() {
			It("uses drax from $PATH", func() {
				newPATH := fmt.Sprintf("%s:%s", tempFolder, os.Getenv("PATH"))
				_, err := Runner.WithoutDraxBin().WithEnvVar(fmt.Sprintf("PATH=%s", newPATH)).Create(spec)
				Expect(err).ToNot(HaveOccurred())

				contents, err := ioutil.ReadFile(draxCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - drax"))
			})
		})
	})

	Context("when inclusive disk limit is provided", func() {
		BeforeEach(func() {
			Expect(writeMegabytes(filepath.Join(sourceImagePath, "fatfile"), 5)).To(Succeed())
		})

		Describe("--btrfs-progs-path global flag", func() {
			var (
				btrfsCalledFile *os.File
				btrfsProgsPath  string
			)

			BeforeEach(func() {
				btrfsProgsPath, _, btrfsCalledFile = integration.CreateFakeBin("btrfs")
			})

			Context("when it's provided", func() {
				It("uses calls the provided btrfs binary", func() {
					_, err := Runner.WithBtrfsProgsPath(btrfsProgsPath).Create(spec)
					Expect(err).To(HaveOccurred())

					contents, err := ioutil.ReadFile(btrfsCalledFile.Name())
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("I'm groot - btrfs"))
				})
			})

			Context("when it's not provided", func() {
				It("uses btrfs from $PATH", func() {
					newPATH := fmt.Sprintf("%s:%s", btrfsProgsPath, os.Getenv("PATH"))
					Runner.WithEnvVar(fmt.Sprintf("PATH=%s", newPATH)).Create(spec)

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
				btrfsProgsPath  string
			)

			BeforeEach(func() {
				btrfsProgsPath, _, btrfsCalledFile = integration.CreateFakeBin("btrfs")
				cfg := config.Config{BtrfsProgsPath: btrfsProgsPath}
				Expect(Runner.SetConfig(cfg)).To(Succeed())
			})

			It("uses the btrfs bin from the config file", func() {
				_, err := Runner.Create(spec)
				Expect(err).To(HaveOccurred())

				contents, err := ioutil.ReadFile(btrfsCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - btrfs"))
			})
		})
	})
})

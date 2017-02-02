package groot_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"

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
})

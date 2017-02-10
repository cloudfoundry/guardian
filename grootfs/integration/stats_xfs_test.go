package integration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Stats (xfs only)", func() {
	var (
		baseImagePath   string
		sourceImagePath string
	)

	const imageId = "random-id"

	BeforeEach(func() {
		integration.SkipIfNotXFS(Driver)

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

		_, err := Runner.Create(groot.CreateSpec{
			BaseImage: baseImagePath,
			ID:        imageId,
			DiskLimit: int64(1024 * 10000),
		})
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("--xfsprogs-path global flag", func() {
		var (
			xfsQuotaCalledFile *os.File
			xfsQuotaBin        *os.File
			xfsProgsPath       string
		)

		BeforeEach(func() {
			xfsProgsPath, xfsQuotaBin, xfsQuotaCalledFile = integration.CreateFakeBin("xfs_quota")
		})

		AfterEach(func() {
			Expect(os.RemoveAll(xfsProgsPath)).To(Succeed())
		})

		Context("when it's provided", func() {
			It("uses the provided xfs progs path", func() {
				_, err := Runner.WithXFSProgsPath(xfsProgsPath).Stats(imageId)
				Expect(err).To(HaveOccurred())

				contents, err := ioutil.ReadFile(xfsQuotaCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - xfs_quota"))
			})
		})

		Context("when it's not provided", func() {
			It("uses xfs from $PATH", func() {
				newPATH := fmt.Sprintf("%s:%s", xfsProgsPath, os.Getenv("PATH"))
				_, err := Runner.WithEnvVar(fmt.Sprintf("PATH=%s", newPATH)).WithXFSProgsPath(xfsProgsPath).Stats(imageId)
				Expect(err).To(HaveOccurred())

				contents, err := ioutil.ReadFile(xfsQuotaCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - xfs_quota"))
			})
		})
	})

	Describe("--config global flag", func() {
		Describe("xfs progs path", func() {
			var (
				xfsQuotaCalledFile *os.File
				xfsQuotaBin        *os.File
				xfsProgsPath       string
			)

			BeforeEach(func() {
				xfsProgsPath, xfsQuotaBin, xfsQuotaCalledFile = integration.CreateFakeBin("xfs_quota")
				cfg := config.Config{XFSProgsPath: xfsProgsPath}
				Expect(Runner.SetConfig(cfg)).To(Succeed())
			})

			It("uses the xfs progs path from the config file", func() {
				_, err := Runner.Stats(imageId)
				Expect(err).To(HaveOccurred())

				contents, err := ioutil.ReadFile(xfsQuotaCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - xfs_quota"))
			})
		})
	})
})

package integration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Create (overlay-xfs only)", func() {
	var (
		randomImageID   string
		baseImagePath   string
		sourceImagePath string
		spec            groot.CreateSpec
	)

	BeforeEach(func() {
		var err error
		sourceImagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		Expect(ioutil.WriteFile(path.Join(sourceImagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())

		randomImageID = testhelpers.NewRandomID()
	})

	AfterEach(func() {
		Expect(os.RemoveAll(sourceImagePath)).To(Succeed())
		Expect(os.RemoveAll(baseImagePath)).To(Succeed())
	})

	JustBeforeEach(func() {
		baseImageFile := integration.CreateBaseImageTar(sourceImagePath)
		baseImagePath = baseImageFile.Name()

		spec = groot.CreateSpec{
			BaseImageURL: integration.String2URL(baseImagePath),
			ID:           randomImageID,
			Mount:        mountByDefault(),
			DiskLimit:    tenMegabytes,
		}
	})

	Describe("--tardis-bin global flag", func() {
		var (
			tardisCalledFile *os.File
			tardisBin        *os.File
			tempFolder       string
		)

		BeforeEach(func() {
			tempFolder, tardisBin, tardisCalledFile = integration.CreateFakeTardis()
		})

		Context("when it's provided", func() {
			It("uses the provided tardis", func() {
				_, err := Runner.WithTardisBin(tardisBin.Name()).Create(spec)
				Expect(err).NotTo(HaveOccurred())

				contents, err := ioutil.ReadFile(tardisCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - tardis"))
			})

			Context("when the tardis bin doesn't have uid bit set", func() {
				BeforeEach(func() {
					testhelpers.UnsuidBinary(tardisBin.Name())
				})

				Context("and groot is running rootless", func() {
					BeforeEach(func() {
						integration.SkipIfRoot(GrootfsTestUid)
					})

					It("returns a sensible error", func() {
						_, err := Runner.WithTardisBin(tardisBin.Name()).Create(spec)
						Expect(err.Error()).To(ContainSubstring("missing the setuid bit on tardis"))
					})

					It("doesn't leak the image dir", func() {
						_, err := Runner.WithTardisBin(tardisBin.Name()).Create(spec)
						Expect(err).To(HaveOccurred())

						imagePath := path.Join(Runner.StorePath, "images", randomImageID)
						Expect(imagePath).ToNot(BeAnExistingFile())
					})
				})

				Context("but groot is running as root", func() {
					BeforeEach(func() {
						integration.SkipIfNonRoot(GrootfsTestUid)
					})

					It("succeeds anyway", func() {
						_, err := Runner.WithTardisBin(tardisBin.Name()).Create(spec)
						Expect(err).NotTo(HaveOccurred())
					})
				})
			})
		})

		Context("when it's not provided", func() {
			It("uses tardis from $PATH", func() {
				newPATH := fmt.Sprintf("%s:%s", tempFolder, os.Getenv("PATH"))
				_, err := Runner.WithoutTardisBin().WithEnvVar(fmt.Sprintf("PATH=%s", newPATH)).Create(spec)
				Expect(err).ToNot(HaveOccurred())

				contents, err := ioutil.ReadFile(tardisCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - tardis"))
			})
		})
	})
})

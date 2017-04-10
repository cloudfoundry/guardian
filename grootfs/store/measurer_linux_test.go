package store_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Measurer", func() {
	var (
		storePath     string
		storeMeasurer *store.StoreMeasurer
		logger        lager.Logger
	)

	BeforeEach(func() {
		var err error

		storePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.MkdirAll(
			filepath.Join(storePath, store.CacheDirName), 0744,
		)).To(Succeed())
		Expect(os.MkdirAll(
			filepath.Join(storePath, store.VolumesDirName), 0744,
		)).To(Succeed())
		Expect(os.MkdirAll(
			filepath.Join(storePath, store.ImageDirName), 0744,
		)).To(Succeed())

		storeMeasurer = store.NewStoreMeasurer(storePath)

		logger = lagertest.NewTestLogger("store-measurer")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
	})

	It("measures space used by the blobs cache and volumes", func() {
		blobsPath := filepath.Join(storePath, store.CacheDirName)
		Expect(writeFile(filepath.Join(blobsPath, "sha256:fake"), 256*1024)).To(Succeed())

		volPath := filepath.Join(storePath, store.VolumesDirName, "sha256:fake")
		Expect(os.MkdirAll(volPath, 0744)).To(Succeed())
		Expect(writeFile(filepath.Join(volPath, "my-file"), 256*1024)).To(Succeed())

		imagePath := filepath.Join(storePath, store.ImageDirName, "my-image")
		Expect(os.MkdirAll(imagePath, 0744)).To(Succeed())
		Expect(writeFile(filepath.Join(imagePath, "my-file"), 256*1024)).To(Succeed())

		storeSize, err := storeMeasurer.MeasureStore(logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(storeSize).To(BeNumerically("~", 512*1024, 1024))
	})

	Context("when the store does not exist", func() {
		BeforeEach(func() {
			storeMeasurer = store.NewStoreMeasurer("/path/to/non/existent/store")
		})

		It("returns a useful error", func() {
			_, err := storeMeasurer.MeasureStore(logger)
			Expect(err).To(MatchError(ContainSubstring("No such file or directory")))
		})
	})

	Context("when the volume path does not exist", func() {
		BeforeEach(func() {
			Expect(os.RemoveAll(filepath.Join(storePath, store.VolumesDirName))).To(Succeed())
		})

		It("returns a useful error", func() {
			_, err := storeMeasurer.MeasureStore(logger)
			Expect(err).To(MatchError(ContainSubstring("No such file or directory")))
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

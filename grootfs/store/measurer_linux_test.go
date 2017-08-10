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

var _ = FDescribe("Measurer", func() {
	var (
		storePath     string
		storeMeasurer *store.StoreMeasurer
		logger        lager.Logger
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

		storeMeasurer = store.NewStoreMeasurer(storePath)

		logger = lagertest.NewTestLogger("store-measurer")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
	})

	It("measures space used by the volumes and images", func() {
		volPath := filepath.Join(storePath, store.VolumesDirName, "sha256:fake")
		Expect(os.MkdirAll(volPath, 0744)).To(Succeed())
		Expect(writeFile(filepath.Join(volPath, "my-file"), 2048*1024)).To(Succeed())

		imagePath := filepath.Join(storePath, store.ImageDirName, "my-image")
		Expect(os.MkdirAll(imagePath, 0744)).To(Succeed())
		Expect(writeFile(filepath.Join(imagePath, "my-file"), 2048*1024)).To(Succeed())

		storeSize, err := storeMeasurer.MeasureStore(logger)
		Expect(err).NotTo(HaveOccurred())
		xfsMetadataSize := 33755136
		Expect(storeSize).To(BeNumerically("~", 4096*1024+xfsMetadataSize, 256*1024))
	})

	Context("when the store does not exist", func() {
		BeforeEach(func() {
			storeMeasurer = store.NewStoreMeasurer("/path/to/non/existent/store")
		})

		It("returns a useful error", func() {
			_, err := storeMeasurer.MeasureStore(logger)
			Expect(err).To(MatchError(ContainSubstring("Invalid path /path/to/non/existent/store")))
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

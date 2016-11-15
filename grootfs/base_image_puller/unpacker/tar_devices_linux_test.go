package unpacker_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Tar unpacker - Handling devices", func() {
	var (
		tarUnpacker   *unpacker.TarUnpacker
		logger        lager.Logger
		baseImagePath string
		stream        *gbytes.Buffer
		targetPath    string
	)

	BeforeEach(func() {
		tarUnpacker = unpacker.NewTarUnpacker()

		var err error
		targetPath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		baseImagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		logger = lagertest.NewTestLogger("test-store")
	})

	JustBeforeEach(func() {
		stream = gbytes.NewBuffer()
		sess, err := gexec.Start(exec.Command("tar", "-c", "-C", baseImagePath, "."), stream, nil)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))
	})

	AfterEach(func() {
		Expect(os.RemoveAll(baseImagePath)).To(Succeed())
		Expect(os.RemoveAll(targetPath)).To(Succeed())
	})

	Context("when there are device files", func() {
		BeforeEach(func() {
			Expect(exec.Command("sudo", "mknod", path.Join(baseImagePath, "a_device"), "c", "1", "8").Run()).To(Succeed())
		})

		It("excludes them", func() {
			Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})).To(Succeed())

			filePath := path.Join(targetPath, "a_device")
			Expect(filePath).ToNot(BeAnExistingFile())
		})
	})
})

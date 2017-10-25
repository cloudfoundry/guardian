package gqt_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Partially shared containers (peas)", func() {
	var (
		gdn       *runner.RunningGarden
		tmpDir    string
		peaRootfs string
		ctr       garden.Container
	)

	BeforeEach(func() {
		gdn = runner.Start(config)
		var err error
		tmpDir, err = ioutil.TempDir("", "peas-gqts")
		Expect(err).NotTo(HaveOccurred())

		Expect(exec.Command("cp", "-a", defaultTestRootFS, tmpDir).Run()).To(Succeed())
		Expect(os.Chmod(tmpDir, 0777)).To(Succeed())
		peaRootfs = filepath.Join(tmpDir, "rootfs")
		Expect(ioutil.WriteFile(filepath.Join(peaRootfs, "ima-pea"), []byte("pea!"), 0644)).To(Succeed())

		ctr, err = gdn.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(gdn.DestroyAndStop()).To(Succeed())
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	It("should not leak pipes", func() {
		initialPipes := numPipes(gdn.Pid)

		process, err := ctr.Run(garden.ProcessSpec{
			Path:  "echo",
			Args:  []string{"hello"},
			Image: garden.ImageRef{URI: "raw://" + peaRootfs},
		}, garden.ProcessIO{})
		Expect(err).NotTo(HaveOccurred())
		Expect(process.Wait()).To(Equal(0))

		Eventually(func() int { return numPipes(gdn.Pid) }).Should(Equal(initialPipes))
	})
})

package gqt_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Init process", func() {
	var (
		tmpDir        string
		parentCommand *exec.Cmd
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		cmd := exec.Command("gcc", "-static", "-o", "test_init", "test_init.c", "../../cmd/init/ignore_sigchild.c", "-I", "../../cmd/init")
		runCommandInDir(cmd, "cmd")
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	It("does not allow children to become zombies", func() {
		parentCommand = exec.Command("cmd/test_init")
		parentCommand.Start()

		time.Sleep(1)

		Eventually(countPsOccurances).Should(Equal(1))
		Expect(string(runPs())).NotTo(ContainSubstring("defunct"))
	})
})

func countPsOccurances() int {
	psout := runPs()

	testInitRe := regexp.MustCompile("test_init")

	matches := testInitRe.FindAll(psout, -1)

	return len(matches)
}

func runPs() []byte {

	cmd := exec.Command("ps", "auxf")
	cmd.Wait()

	psout, err := cmd.Output()
	Expect(err).NotTo(HaveOccurred())

	return psout

}

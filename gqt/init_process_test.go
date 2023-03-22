package gqt_test

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Init process", func() {
	var (
		tmpDir        string
		parentCommand *exec.Cmd
	)

	BeforeEach(func() {
		tmpDir = tempDir("", "")

		cmd := exec.Command("gcc", "-static", "-o", "test_init", "test_init.c", "../../cmd/init/ignore_sigchild.c", "-I", "../../cmd/init")
		runCommandInDir(cmd, "cmd")
	})

	AfterEach(func() {
		Expect(parentCommand.Process.Kill()).To(Succeed())
		_, waitErr := parentCommand.Process.Wait()
		Expect(waitErr).NotTo(HaveOccurred())
		os.RemoveAll(tmpDir)
	})

	It("does not allow children to become zombies", func() {
		parentCommand = exec.Command("cmd/test_init")
		Expect(parentCommand.Start()).To(Succeed())

		Eventually(countPsOccurances).Should(Equal(1))

		psOut := string(runPs())
		matchingPsLines := []string{}
		psLines := strings.Split(psOut, "\n")
		for _, psLine := range psLines {
			if !strings.Contains(psLine, "test_init") {
				continue
			}
			matchingPsLines = append(matchingPsLines, psLine)
		}

		Expect(strings.Join(matchingPsLines, "\n")).NotTo(ContainSubstring("defunct"), fmt.Sprintf("\n\nps output:\n%s\n\n", psOut))
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
	psout, err := cmd.Output()
	Expect(err).NotTo(HaveOccurred())

	return psout
}

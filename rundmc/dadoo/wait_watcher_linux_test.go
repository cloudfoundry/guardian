package dadoo_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/cloudfoundry-incubator/guardian/rundmc/dadoo"
	"github.com/docker/docker/pkg/reexec"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

func init() {
	reexec.Register("fadoo", func() {
		dadoo.Listen(os.Args[1])
		fmt.Println("listening")

		select {}
	})

	if reexec.Init() {
		os.Exit(0)
	}
}

var _ = Describe("WaitWatcher", func() {
	var (
		ww       *dadoo.WaitWatcher
		sockDir  string
		sockPath string

		fakeDadoo *gexec.Session
	)

	BeforeEach(func() {
		var err error

		ww = &dadoo.WaitWatcher{}

		sockDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		sockPath = path.Join(sockDir, "exit.sock")
		Expect(err).NotTo(HaveOccurred())

		cmd := reexec.Command("fadoo", sockPath)
		fakeDadoo, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(fakeDadoo).Should(gbytes.Say("listening"))
	})

	AfterEach(func() {
		Expect(os.RemoveAll(sockDir)).To(Succeed())
		fakeDadoo.Kill()
	})

	It("should wait for the listening process to exit", func() {
		ch, err := ww.Wait(sockPath)
		Expect(err).NotTo(HaveOccurred())

		Consistently(ch).ShouldNot(BeClosed())
		fakeDadoo.Kill()
		Eventually(fakeDadoo).Should(gexec.Exit())
		Eventually(ch).Should(BeClosed())
	})

	It("should immediately exit if the listening process already died", func() {
		fakeDadoo.Kill()
		Eventually(fakeDadoo).Should(gexec.Exit())

		_, err := ww.Wait(sockPath)
		Expect(err).To(MatchError(ContainSubstring("connection refused")))
	})

	It("should immediately exit with an error if the socket path does not exist", func() {
		fakeDadoo.Kill()
		Eventually(fakeDadoo).Should(gexec.Exit())

		_, err := ww.Wait("not-a-real-path.sock")
		Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
	})
})

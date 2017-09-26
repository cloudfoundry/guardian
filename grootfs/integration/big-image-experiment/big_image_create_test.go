package integration_test

import (
	"fmt"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"code.cloudfoundry.org/grootfs/groot"
	runnerpkg "code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/grootfs/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Create with remote DOCKER images", func() {
	var (
		randomImageID string
		runner        runnerpkg.Runner
	)

	BeforeEach(func() {
		initSpec := runnerpkg.InitSpec{UIDMappings: []groot.IDMappingSpec{
			{HostID: GrootUID, NamespaceID: 0, Size: 1},
			{HostID: 100000, NamespaceID: 1, Size: 65000},
		},
			GIDMappings: []groot.IDMappingSpec{
				{HostID: GrootGID, NamespaceID: 0, Size: 1},
				{HostID: 100000, NamespaceID: 1, Size: 65000},
			},
		}

		randomImageID = testhelpers.NewRandomID()
		Expect(Runner.RunningAsUser(0, 0).InitStore(initSpec)).To(Succeed())
		runner = Runner.SkipInitStore()
	})

	Context("when using the default registry", func() {
		var tcpDumpSess *gexec.Session

		BeforeEach(func() {
			var err error
			tcpDumpSess, err = gexec.Start(exec.Command("tcpdump", "-U", "-w", "/tmp/packets", "tcp"), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			tcpDumpSess.Kill()
		})

		It("doesn't fail", func() {
			sess, err := runner.StartCreate(groot.CreateSpec{
				BaseImage: "docker:///ubuntu:trusty",
				ID:        randomImageID,
				Mount:     mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())

			deadline := time.Now().Add(60 * time.Second)
			for {
				if sess.ExitCode() != -1 {
					break
				}
				if time.Now().After(deadline) {
					fmt.Println(">>>> printing debug info")

					fmt.Println(">>>> netstat:")
					netstatCmd := exec.Command("bash", "-c", "netstat -tanp | grep "+strconv.Itoa(sess.Command.Process.Pid))
					netstatCmd.Stdout = GinkgoWriter
					netstatCmd.Stderr = GinkgoWriter
					Expect(netstatCmd.Run()).To(Succeed())

					fmt.Println(">>>> tcpdump:")
					cmd := exec.Command("tcpdump", "-r", "/tmp/packets", "(src registry-1.docker.io or dst registry-1.docker.io) and tcp[tcpflags] & (tcp-ack) != 0")
					cmd.Stdout = GinkgoWriter
					cmd.Stderr = GinkgoWriter
					Expect(cmd.Run()).To(Succeed())
					fmt.Println(">>>> if this build recently failed, please hijack the container and download `/tmp/packets`, which contains more tcpdump info")

					fmt.Println(">>>> goroutine stack:")
					sess.Signal(syscall.SIGQUIT)
					Fail("timeout exeeded")
				}
				time.Sleep(100 * time.Millisecond)
			}
			Expect(sess.ExitCode()).To(Equal(0))
		})
	})
})

package signals_test

import (
	"errors"
	"os/exec"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/signals"
	"code.cloudfoundry.org/guardian/rundmc/signals/signalsfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Signaller", func() {
	var (
		signaller signals.Signaller
		pidGetter *signalsfakes.FakePidGetter
		proc      *gexec.Session
	)

	BeforeEach(func() {
		cmd := exec.Command("bash", "-c", `
trap "echo terminated; exit 42" SIGTERM
echo ready
while true; do
	sleep 0.1
done
`)

		var err error
		proc, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(proc).Should(gbytes.Say("ready"))

		pidGetter = new(signalsfakes.FakePidGetter)
		pidGetter.PidReturns(proc.Command.Process.Pid, nil)
		signallerFactory := &signals.SignallerFactory{PidGetter: pidGetter}
		signaller = signallerFactory.NewSignaller("some-pid-path")
	})

	AfterEach(func() {
		// Don't check error, a successful test should have already killed this process
		proc.Kill()
	})

	It("gets the pid from the pidfile", func() {
		Expect(signaller.Signal(garden.SignalTerminate)).To(Succeed())
		Expect(pidGetter.PidCallCount()).To(Equal(1))
		Expect(pidGetter.PidArgsForCall(0)).To(Equal("some-pid-path"))
	})

	It("forwards SIGTERM", func() {
		Expect(signaller.Signal(garden.SignalTerminate)).To(Succeed())
		Expect(proc.Wait()).To(gexec.Exit(42))
		Expect(string(proc.Buffer().Contents())).To(ContainSubstring("terminated"))
	})

	It("forwards SIGKILL", func() {
		Expect(signaller.Signal(garden.SignalKill)).To(Succeed())
		Eventually(proc, time.Second*3).Should(gexec.Exit(137))
		Expect(string(proc.Buffer().Contents())).NotTo(ContainSubstring("terminated"))
	})

	Context("when the pidgetter returns an error", func() {
		BeforeEach(func() {
			pidGetter.PidReturns(0, errors.New("pid-lookup-error"))
		})

		It("returns a wrapped error", func() {
			Expect(signaller.Signal(garden.SignalKill)).To(MatchError(ContainSubstring("pid-lookup-error")))
		})
	})

	Context("when no process with specified pid is running", func() {
		BeforeEach(func() {
			proc = proc.Kill()
			Expect(proc.Wait()).To(gexec.Exit(137))
		})

		It("returns an error", func() {
			Expect(signaller.Signal(garden.SignalKill)).To(MatchError(ContainSubstring("process already finished")))
		})
	})
})

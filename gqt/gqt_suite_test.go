package gqt_test

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var OciRuntimeBin = os.Getenv("OCI_RUNTIME")

var defaultRuntime = map[string]string{
	"linux": "runc",
}

var ginkgoIO = garden.ProcessIO{Stdout: GinkgoWriter, Stderr: GinkgoWriter}

func TestGqt(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeEach(func() {
		if OciRuntimeBin == "" {
			Skip("No OCI Runtime for Platform: " + runtime.GOOS)
		}
	})

	SetDefaultEventuallyTimeout(5 * time.Second)
	RunSpecs(t, "GQT Suite")
}

var gardenBin, iodaemonBin string

var _ = BeforeSuite(func() {
	if OciRuntimeBin == "" {
		OciRuntimeBin = defaultRuntime[runtime.GOOS]
	}

	if OciRuntimeBin == "" {
		fmt.Fprintf(GinkgoWriter, "Skipping GQT BeforeSuite, no OCI runtime for %q\n", runtime.GOOS)
		return
	}

	var err error
	gardenBin, err = gexec.Build("github.com/cloudfoundry-incubator/guardian/cmd/guardian", "-tags", "daemon")
	Expect(err).NotTo(HaveOccurred())
	iodaemonBin, err = gexec.Build("github.com/cloudfoundry-incubator/guardian/rundmc/iodaemon/cmd/iodaemon")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

func startGarden(argv ...string) *runner.RunningGarden {
	return runner.Start(gardenBin, iodaemonBin, argv...)
}

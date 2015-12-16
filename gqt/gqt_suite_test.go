package gqt_test

import (
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
			OciRuntimeBin = defaultRuntime[runtime.GOOS]
		}

		if OciRuntimeBin == "" {
			Skip("No OCI Runtime for Platform: " + runtime.GOOS)
		}
	})

	SetDefaultEventuallyTimeout(5 * time.Second)
	RunSpecs(t, "GQT Suite")
}

func startGarden(argv ...string) *runner.RunningGarden {
	gardenBin, err := gexec.Build("github.com/cloudfoundry-incubator/guardian/cmd/guardian", "-tags", "daemon")
	Expect(err).NotTo(HaveOccurred())

	iodaemonBin, err := gexec.Build("github.com/cloudfoundry-incubator/guardian/rundmc/iodaemon/cmd/iodaemon")
	Expect(err).NotTo(HaveOccurred())

	if networkModule := os.Getenv("NETWORK_MODULE"); networkModule != "" {
		argv = append(argv, "--networkModule="+networkModule)
	}

	return runner.Start(gardenBin, iodaemonBin, argv...)
}

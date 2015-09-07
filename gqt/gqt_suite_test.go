package gqt_test

import (
	"os"
	"runtime"

	"github.com/cloudfoundry-incubator/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var OciRuntimeBin = os.Getenv("OCI_RUNTIME")

var defaultRuntime = map[string]string{
	"linux":  "runc",
	"darwin": "nocs",
}

func TestGqt(t *testing.T) {
	RegisterFailHandler(Fail)

	if OciRuntimeBin == "" {
		OciRuntimeBin = defaultRuntime[runtime.GOOS]
	}

	RunSpecs(t, "Gqt Suite")
}

func startGarden(argv ...string) *runner.RunningGarden {
	gardenBin, err := gexec.Build("github.com/cloudfoundry-incubator/guardian/cmd/guardian")
	Expect(err).NotTo(HaveOccurred())

	return runner.Start(gardenBin, argv...)
}

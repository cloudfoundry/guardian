package gqt_test

import (
	"os"
	"os/exec"
	"path"
	"runtime"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"encoding/json"
	"testing"
)

var defaultRuntime = map[string]string{
	"linux": "runc",
}

var ginkgoIO = garden.ProcessIO{Stdout: GinkgoWriter, Stderr: GinkgoWriter}

var ociRuntimeBin, gardenBin, initBin, kawasakiBin, iodaemonBin, nstarBin, dadooBin, inspectorGardenBin string

func TestGqt(t *testing.T) {
	RegisterFailHandler(Fail)

	SynchronizedBeforeSuite(func() []byte {
		var err error
		bins := make(map[string]string)

		bins["oci_runtime_path"] = os.Getenv("OCI_RUNTIME")
		if bins["oci_runtime_path"] == "" {
			bins["oci_runtime_path"] = defaultRuntime[runtime.GOOS]
		}

		if bins["oci_runtime_path"] != "" {
			bins["garden_bin_path"], err = gexec.Build("github.com/cloudfoundry-incubator/guardian/cmd/guardian", "-tags", "daemon")
			Expect(err).NotTo(HaveOccurred())

			bins["kawasaki_bin_path"], err = gexec.Build("github.com/cloudfoundry-incubator/guardian/cmd/kawasaki")
			Expect(err).NotTo(HaveOccurred())

			bins["iodaemon_bin_path"], err = gexec.Build("github.com/cloudfoundry-incubator/guardian/rundmc/iodaemon/cmd/iodaemon")
			Expect(err).NotTo(HaveOccurred())

			bins["dadoo_bin_bin_bin"], err = gexec.Build("github.com/cloudfoundry-incubator/guardian/cmd/dadoo")
			Expect(err).NotTo(HaveOccurred())

			bins["init_bin_path"], err = gexec.Build("github.com/cloudfoundry-incubator/guardian/cmd/init")
			Expect(err).NotTo(HaveOccurred())

			bins["inspector-garden_bin_path"], err = gexec.Build("github.com/cloudfoundry-incubator/guardian/cmd/inspector-garden")
			Expect(err).NotTo(HaveOccurred())

			cmd := exec.Command("make")
			cmd.Dir = "../rundmc/nstar"
			cmd.Stdout = GinkgoWriter
			cmd.Stderr = GinkgoWriter
			Expect(cmd.Run()).To(Succeed())
			bins["nstar_bin_path"] = "../rundmc/nstar/nstar"
		}

		data, err := json.Marshal(bins)
		Expect(err).NotTo(HaveOccurred())

		return data
	}, func(data []byte) {
		bins := make(map[string]string)
		Expect(json.Unmarshal(data, &bins)).To(Succeed())

		ociRuntimeBin = bins["oci_runtime_path"]
		gardenBin = bins["garden_bin_path"]
		iodaemonBin = bins["iodaemon_bin_path"]
		nstarBin = bins["nstar_bin_path"]
		dadooBin = bins["dadoo_bin_bin_bin"]
		kawasakiBin = bins["kawasaki_bin_path"]
		initBin = bins["init_bin_path"]
		inspectorGardenBin = bins["inspector-garden_bin_path"]
	})

	BeforeEach(func() {
		if ociRuntimeBin == "" {
			Skip("No OCI Runtime for Platform: " + runtime.GOOS)
		}

		if os.Getenv("GARDEN_TEST_ROOTFS") == "" {
			Skip("No Garden RootFS")
		}

		Expect(os.Chmod(initBin, 0755)).To(Succeed())
		Expect(os.Chmod(path.Dir(initBin), 0755)).To(Succeed())
		Expect(os.Chmod(path.Dir(path.Dir(initBin)), 0755)).To(Succeed())
	})

	SetDefaultEventuallyTimeout(5 * time.Second)
	RunSpecs(t, "GQT Suite")
}

func startGarden(argv ...string) *runner.RunningGarden {
	return runner.Start(gardenBin, initBin, kawasakiBin, iodaemonBin, nstarBin, dadooBin, argv...)
}

package gqt_test

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Partially shared containers (peas)", func() {
	var gdn *runner.RunningGarden

	BeforeEach(func() {
		gdn = runner.Start(config)
	})

	AfterEach(func() {
		Expect(gdn.DestroyAndStop()).To(Succeed())
	})

	It("can run a process in its own mount namespace", func() {
		ctr, err := gdn.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())
		proc, err := ctr.Run(garden.ProcessSpec{
			Path:  "sleep",
			Args:  []string{"1000"},
			User:  "root",
			Image: garden.ImageRef{URI: "not-used-yet"},
		}, garden.ProcessIO{
			Stdout: GinkgoWriter,
			Stderr: GinkgoWriter,
		})
		Expect(err).NotTo(HaveOccurred())

		ctrInitPid := readFile(filepath.Join(gdn.DepotDir, ctr.Handle(), "pidfile"))
		sleepProcessPid := readFile(filepath.Join(gdn.DepotDir, ctr.Handle(), "processes", proc.ID(), "pidfile"))

		ctrInitMntNS := getNS(ctrInitPid, "mnt")
		sleepProcessMntNs := getNS(sleepProcessPid, "mnt")
		Expect(sleepProcessMntNs).NotTo(Equal(ctrInitMntNS))
	})
})

func getNS(pid string, ns string) string {
	ns, err := os.Readlink(fmt.Sprintf("/proc/%s/ns/%s", string(pid), ns))
	Expect(err).NotTo(HaveOccurred())
	return ns
}

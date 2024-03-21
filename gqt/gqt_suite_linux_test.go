//go:build linux

package gqt_test

import (
	"os"
	"strconv"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/gomega"
)

var (
	// the unprivileged user is baked into the cloudfoundry/garden-runc-release image
	unprivilegedUID = uint32(5000)
	unprivilegedGID = uint32(5000)
)

func restartGarden(client *runner.RunningGarden, config runner.GdnRunnerConfig) *runner.RunningGarden {
	Expect(client.Ping()).To(Succeed(), "tried to restart garden while it was not running")
	Expect(client.Stop()).To(Succeed())
	return runner.Start(config)
}

func idToStr(id uint32) string {
	return strconv.FormatUint(uint64(id), 10)
}

func removeSocket() {
	_, err := os.Stat(config.BindSocket)
	if err == nil {
		Expect(os.Remove(config.BindSocket)).To(Succeed())
	} else if !os.IsNotExist(err) {
		Expect(err).NotTo(HaveOccurred())
	}
}

func createRootfsTar(modifyRootfs func(string)) string {
	return tarUpDir(createRootfs(modifyRootfs, 0755))
}

func uint32ptr(i uint32) *uint32 {
	return &i
}

func runInContainer(container garden.Container, path string, args []string) {
	runInContainerWithIO(container, ginkgoIO, path, args)
}

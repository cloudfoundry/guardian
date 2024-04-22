//go:build linux

package gqt_test

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/gomega"
)

func restartGarden(client *runner.RunningGarden, config runner.GdnRunnerConfig) *runner.RunningGarden {
	Expect(client.Ping()).To(Succeed(), "tried to restart garden while it was not running")
	Expect(client.Stop()).To(Succeed())
	return runner.Start(config)
}

func createRootfsTar(modifyRootfs func(string)) string {
	return tarUpDir(createRootfs(modifyRootfs, 0755))
}

func runInContainer(container garden.Container, path string, args []string) {
	runInContainerWithIO(container, ginkgoIO, path, args)
}

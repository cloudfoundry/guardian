package runner

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type UserCredential interface{}

func cmd(tmpdir, depotDir, graphPath, consoleSocketsPath, network, addr string, binaries *Binaries, rootfs string, user UserCredential, argv ...string) *exec.Cmd {
	Expect(os.MkdirAll(tmpdir, 0755)).To(Succeed())
	Expect(os.MkdirAll(depotDir, 0755)).To(Succeed())

	appendDefaultFlag := func(ar []string, key, value string) []string {
		for _, a := range argv {
			if a == key {
				return ar
			}
		}

		if value != "" {
			return append(ar, key, value)
		} else {
			return append(ar, key)
		}
	}

	gardenArgs := make([]string, len(argv))
	copy(gardenArgs, argv)

	switch network {
	case "tcp":
		gardenArgs = appendDefaultFlag(gardenArgs, "--bind-ip", strings.Split(addr, ":")[0])
		gardenArgs = appendDefaultFlag(gardenArgs, "--bind-port", strings.Split(addr, ":")[1])
	case "unix":
		gardenArgs = appendDefaultFlag(gardenArgs, "--bind-socket", addr)
	}

	if rootfs != "" {
		gardenArgs = appendDefaultFlag(gardenArgs, "--default-rootfs", rootfs)
	}

	gardenArgs = appendDefaultFlag(gardenArgs, "--depot", depotDir)
	gardenArgs = appendDefaultFlag(gardenArgs, "--tag", fmt.Sprintf("%d", GinkgoParallelNode()))

	gardenArgs = appendDefaultFlag(gardenArgs, "--network-plugin", binaries.NoopPlugin)
	gardenArgs = appendDefaultFlag(gardenArgs, "--image-plugin", binaries.NoopPlugin)

	return exec.Command(binaries.Gdn, append([]string{"server"}, gardenArgs...)...)
}

func (r *RunningGarden) Cleanup() {
	if runtime.GOOS == "linux" {
		MustUnmountTmpfs(r.GraphPath)
	}

	// In the kernel version 3.19.0-51-generic the code bellow results in
	// hanging the running VM. We are not deleting the node-X directories. They
	// are empty and the next test will re-use them. We will stick with that
	// workaround until we can test on a newer kernel that will hopefully not
	// have this bug.
	//
	// if err := os.RemoveAll(r.GraphPath); err != nil {
	// 	r.logger.Error("remove-graph", err)
	// }

	r.logger.Info("cleanup-tempdirs")
	if err := os.RemoveAll(r.Tmpdir); err != nil {
		r.logger.Error("cleanup-tempdirs-failed", err, lager.Data{"tmpdir": r.Tmpdir})
	} else {
		r.logger.Info("tempdirs-removed")
	}
}

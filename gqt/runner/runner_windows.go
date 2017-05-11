package runner

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type UserCredential interface{}

func cmd(tmpdir, depotDir, graphPath, consoleSocketsPath, network, addr, bin, initBin, nstarBin, dadooBin, grootfsBin, tarBin, rootfs string, user UserCredential, argv ...string) *exec.Cmd {
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
	gardenArgs = appendDefaultFlag(gardenArgs, "--graph", graphPath)
	gardenArgs = appendDefaultFlag(gardenArgs, "--console-sockets-path", consoleSocketsPath)
	gardenArgs = appendDefaultFlag(gardenArgs, "--tag", fmt.Sprintf("%d", GinkgoParallelNode()))
	gardenArgs = appendDefaultFlag(gardenArgs, "--network-pool", fmt.Sprintf("10.254.%d.0/22", 4*GinkgoParallelNode()))
	gardenArgs = appendDefaultFlag(gardenArgs, "--init-bin", initBin)
	gardenArgs = appendDefaultFlag(gardenArgs, "--dadoo-bin", dadooBin)
	gardenArgs = appendDefaultFlag(gardenArgs, "--nstar-bin", nstarBin)
	gardenArgs = appendDefaultFlag(gardenArgs, "--tar-bin", tarBin)
	gardenArgs = appendDefaultFlag(gardenArgs, "--port-pool-start", fmt.Sprintf("%d", GinkgoParallelNode()*7000))

	return exec.Command(bin, append([]string{"server"}, gardenArgs...)...)
}

func (r *RunningGarden) Cleanup() {
	MustUnmountTmpfs(r.GraphPath)

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

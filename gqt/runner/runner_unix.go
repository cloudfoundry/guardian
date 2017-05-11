// +build !windows

package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/eapache/go-resiliency/retrier"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type UserCredential *syscall.Credential

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

	cmd := exec.Command(bin, append([]string{"server"}, gardenArgs...)...)
	if user != nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		cmd.SysProcAttr.Credential = user

		uidGid := fmt.Sprintf("%d:%d", user.Uid, user.Gid)
		Eventually(func() error {
			cmd := exec.Command("chown", "-R", uidGid, tmpdir)
			cmd.Stdout = GinkgoWriter
			cmd.Stderr = GinkgoWriter
			return cmd.Run()
		}, "3s", "1s").Should(Succeed())
	}

	return cmd
}

func (r *RunningGarden) Cleanup() {
	// unmount aufs since the docker graph driver leaves this around,
	// otherwise the following commands might fail
	retry := retrier.New(retrier.ConstantBackoff(200, 500*time.Millisecond), nil)

	err := retry.Run(func() error {
		if err := os.RemoveAll(path.Join(r.GraphPath, "aufs")); err == nil {
			return nil // if we can remove it, it's already unmounted
		}

		if err := syscall.Unmount(path.Join(r.GraphPath, "aufs"), MNT_DETACH); err != nil {
			r.logger.Error("failed-unmount-attempt", err)
			return err
		}

		return nil
	})

	if err != nil {
		r.logger.Error("failed-to-unmount", err)
	}

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

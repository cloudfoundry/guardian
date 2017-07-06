// +build !windows

package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"syscall"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/eapache/go-resiliency/retrier"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type UserCredential *syscall.Credential

func setUserCredential(runner *GardenRunner) {
	runner.Command.SysProcAttr = &syscall.SysProcAttr{Credential: runner.User}
}

func (r *GardenRunner) setupDirsForUser() {
	MustMountTmpfs(r.GraphDir)

	if r.Command.SysProcAttr.Credential != nil {
		uidGid := fmt.Sprintf("%d:%d", r.User.Uid, r.User.Gid)
		Eventually(func() error {
			cmd := exec.Command("chown", "-R", uidGid, r.TmpDir)
			cmd.Stdout = GinkgoWriter
			cmd.Stderr = GinkgoWriter
			return cmd.Run()
		}, "3s", "1s").Should(Succeed())
	}
}

func (r *RunningGarden) Cleanup() {
	// unmount aufs since the docker graph driver leaves this around,
	// otherwise the following commands might fail
	retry := retrier.New(retrier.ConstantBackoff(200, 500*time.Millisecond), nil)

	err := retry.Run(func() error {
		if err := os.RemoveAll(path.Join(r.GraphDir, "aufs")); err == nil {
			return nil // if we can remove it, it's already unmounted
		}

		if err := syscall.Unmount(path.Join(r.GraphDir, "aufs"), MNT_DETACH); err != nil {
			r.logger.Error("failed-unmount-attempt", err)
			return err
		}

		return nil
	})

	if err != nil {
		r.logger.Error("failed-to-unmount", err)
	}

	MustUnmountTmpfs(r.GraphDir)

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
	if err := os.RemoveAll(r.TmpDir); err != nil {
		r.logger.Error("cleanup-tempdirs-failed", err, lager.Data{"tmpdir": r.TmpDir})
	} else {
		r.logger.Info("tempdirs-removed")
	}
}

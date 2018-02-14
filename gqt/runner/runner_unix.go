// +build !windows

package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"syscall"
	"time"

	"github.com/eapache/go-resiliency/retrier"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type UserCredential *syscall.Credential

func setUserCredential(runner *GardenRunner) {
	credential := runner.User
	if runner.Socket2meSocketPath != "" {
		// socket2me sets uid/gid
		credential = nil
	}

	runner.Command.SysProcAttr = &syscall.SysProcAttr{Credential: credential}
}

func socket2meCommand(config GdnRunnerConfig) *exec.Cmd {
	return exec.Command(
		config.Socket2meBin,
		append(
			[]string{
				"--socket-path",
				config.Socket2meSocketPath,
				fmt.Sprintf("--uid=%d", config.User.Uid),
				fmt.Sprintf("--gid=%d", config.User.Gid),
				"--socket-uid=0", "--socket-gid=0",
				config.GdnBin, "server",
			},
			config.toFlags()...,
		)...,
	)
}

func (r *GardenRunner) setupDirsForUser() {
	// TODO: Remove this when we get rid of shed
	MustMountTmpfs(*r.GraphDir)

	if r.User != nil {
		uidGid := fmt.Sprintf("%d:%d", r.User.Uid, r.User.Gid)

		cmd := exec.Command("chown", uidGid, r.TmpDir)
		cmd.Stdout = GinkgoWriter
		cmd.Stderr = GinkgoWriter
		Expect(cmd.Run()).To(Succeed())

		Eventually(func() error {
			cmd := exec.Command("chown", "-R", uidGid, r.DepotDir)
			cmd.Stdout = GinkgoWriter
			cmd.Stderr = GinkgoWriter
			return cmd.Run()
		}, "3s", "1s").Should(Succeed())
	}
}

func (r *RunningGarden) Cleanup() {
	// GROOT CLEANUP
	storePath := r.GardenRunner.GdnRunnerConfig.StorePath
	privStorePath := r.GardenRunner.GdnRunnerConfig.PrivilegedStorePath

	clearGrootStore(r.GrootBin, storePath)
	clearGrootStore(r.GrootBin, privStorePath)

	// AUFS CLEANUP
	// TODO: Remove this when we get rid of shed
	retry := retrier.New(retrier.ConstantBackoff(200, 500*time.Millisecond), nil)
	Expect(retry.Run(func() error {
		if err := os.RemoveAll(path.Join(*r.GraphDir, "aufs")); err == nil {
			return nil // if we can remove it, it's already unmounted
		}

		if err := syscall.Unmount(path.Join(*r.GraphDir, "aufs"), MNT_DETACH); err != nil {
			r.logger.Error("failed-unmount-attempt", err)
			return err
		}

		return nil
	})).To(Succeed())

	MustUnmountTmpfs(*r.GraphDir)

	if err := os.RemoveAll(*r.GraphDir); err != nil {
		r.logger.Error("remove-graph", err)
	}
}

func clearGrootStore(grootBin, storePath string) {
	deleteStoreArgs := []string{"--store", storePath, "delete-store"}

	deleteStore := exec.Command(grootBin, deleteStoreArgs...)
	deleteStore.Stdout = GinkgoWriter
	deleteStore.Stderr = GinkgoWriter
	Expect(deleteStore.Run()).To(Succeed())
}

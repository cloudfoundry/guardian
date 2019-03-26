// +build !windows

package runner

import (
	"fmt"
	"os/exec"
	"syscall"

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
				config.GdnBin,
			},
			config.toServerFlags()...,
		)...,
	)
}

func (r *GardenRunner) setupDirsForUser() {
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

func (r *RunningGarden) Cleanup() error {
	// GROOT CLEANUP
	storePath := r.GardenRunner.GdnRunnerConfig.StorePath
	privStorePath := r.GardenRunner.GdnRunnerConfig.PrivilegedStorePath

	if err := clearGrootStore(r.GrootBin, storePath); err != nil {
		return err
	}
	if err := clearGrootStore(r.GrootBin, privStorePath); err != nil {
		return err
	}

	return nil
}

func clearGrootStore(grootBin, storePath string) error {
	deleteStoreArgs := []string{"--store", storePath, "delete-store"}

	// Ignore lsof errors as lsof would return exit code 1 in case it failed, OR did not find any opened files
	lsofOutput, _ := exec.Command("lsof", "-V", "-x", "+D", storePath).CombinedOutput()

	deleteStore := exec.Command(grootBin, deleteStoreArgs...)
	deleteStore.Stdout = GinkgoWriter
	deleteStore.Stderr = GinkgoWriter
	if err := deleteStore.Run(); err != nil {
		return fmt.Errorf("Delete store failed with %s, lsof(%s) output was %s", err.Error(), storePath, string(lsofOutput))
	}
	return nil
}

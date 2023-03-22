package iptables_test

import (
	"bytes"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"testing"

	"code.cloudfoundry.org/guardian/pkg/locksmith"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestIptablesManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IPtables Suite")
}

type FakeLocksmith struct {
	mutex *sync.Mutex

	keyForLastLock      string
	lockReturnsStuff    bool
	lockReturnsUnlocker locksmith.Unlocker
	lockReturnsErr      error

	unlockReturnsStuff bool
	unlockReturnsErr   error
}

func run(cmd *exec.Cmd) (int, string) {
	var buff bytes.Buffer
	cmd.Stdout = io.MultiWriter(&buff, GinkgoWriter)
	cmd.Stderr = GinkgoWriter
	Expect(cmd.Start()).To(Succeed())
	err := cmd.Wait()
	if err == nil {
		return 0, buff.String()
	}
	exitErr := err.(*exec.ExitError)
	return exitErr.Sys().(syscall.WaitStatus).ExitStatus(), buff.String()
}

func runForStdout(cmd *exec.Cmd) string {
	exitCode, stdout := run(cmd)
	Expect(exitCode).To(Equal(0))
	return stdout
}

func NewFakeLocksmith() *FakeLocksmith {
	return &FakeLocksmith{
		mutex: new(sync.Mutex),
	}
}

func (l *FakeLocksmith) Lock(key string) (locksmith.Unlocker, error) {
	l.keyForLastLock = key

	if l.lockReturnsStuff {
		return l.lockReturnsUnlocker, l.lockReturnsErr
	}

	l.mutex.Lock()
	return l, nil
}

func (l *FakeLocksmith) KeyForLastLock() string {
	return l.keyForLastLock
}

func (l *FakeLocksmith) LockReturns(u locksmith.Unlocker, err error) {
	l.lockReturnsStuff = true
	l.lockReturnsUnlocker = u
	l.lockReturnsErr = err
}

func (l *FakeLocksmith) Unlock() error {
	if l.unlockReturnsStuff {
		return l.unlockReturnsErr
	}

	l.mutex.Unlock()
	return nil
}

func (l *FakeLocksmith) UnlockReturns(err error) {
	l.unlockReturnsStuff = true
	l.unlockReturnsErr = err
}

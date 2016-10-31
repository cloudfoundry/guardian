package iptables_test

import (
	"sync"

	"code.cloudfoundry.org/guardian/pkg/locksmith"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
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

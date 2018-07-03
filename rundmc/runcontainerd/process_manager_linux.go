package runcontainerd

import (
	"fmt"
	"syscall"

	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/nerd"
	"code.cloudfoundry.org/lager"
)

type RegularProcessManager struct {
	nerd *nerd.Nerd
}

func NewRegularProcessManager(nerd *nerd.Nerd) *RegularProcessManager {
	return &RegularProcessManager{nerd: nerd}
}

func (m *RegularProcessManager) Wait(log lager.Logger, containerID, processID string) (int, error) {
	return m.nerd.WaitProcess(log, containerID, processID)
}
func (m *RegularProcessManager) Signal(log lager.Logger, containerID, processID string, signal syscall.Signal) error {
	return m.nerd.Signal(log, containerID, processID, signal)
}

type PeaProcessManager struct {
	nerd *nerd.Nerd
}

func NewPeaProcessManager(nerd *nerd.Nerd) *PeaProcessManager {
	return &PeaProcessManager{nerd: nerd}
}

func (m *PeaProcessManager) Wait(log lager.Logger, containerID, processID string) (int, error) {
	return m.nerd.WaitTask(log, containerID)
}

func (m *PeaProcessManager) Signal(log lager.Logger, containerID, processID string, signal syscall.Signal) error {
	return fmt.Errorf("Signalling a pea is not implemented yet")
}

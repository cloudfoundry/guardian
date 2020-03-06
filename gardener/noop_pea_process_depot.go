package gardener

import (
	"code.cloudfoundry.org/lager"
)

type NoopPeaProcessDepot struct {
}

func (d NoopPeaProcessDepot) CreateProcessDir(log lager.Logger, sandboxHandle, processID string) (string, error) {
	return "", nil
}

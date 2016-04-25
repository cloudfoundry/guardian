package gardener

import "github.com/pivotal-golang/lager"

type NoopRestorer struct{}

func (n *NoopRestorer) Restore(_ lager.Logger, handles []string) []string {
	return handles
}

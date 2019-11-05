package stopper

type NoopStopper struct {
}

func NewNoopStopper() *NoopStopper {
	return &NoopStopper{}
}

func (s NoopStopper) Stop() error {
	return nil
}

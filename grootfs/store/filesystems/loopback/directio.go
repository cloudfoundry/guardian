package loopback

//go:generate counterfeiter . LoSetup
type LoSetup interface {
	FindAssociatedLoopDevice(filePath string) (string, error)
	EnableDirectIO(loopdevPath string) error
	DisableDirectIO(loopdevPath string) error
}

type DirectIOEnabler struct {
	LoSetup LoSetup
}

func NewDirectIOEnabler() *DirectIOEnabler {
	return &DirectIOEnabler{LoSetup: &LoSetupWrapper{}}
}

func (io DirectIOEnabler) Configure(path string) error {
	loopbackDev, err := io.LoSetup.FindAssociatedLoopDevice(path)
	if err != nil {
		return err
	}

	return io.LoSetup.EnableDirectIO(loopbackDev)
}

type DirectIODisabler struct {
	LoSetup LoSetup
}

func NewDirectIODisabler() *DirectIODisabler {
	return &DirectIODisabler{LoSetup: &LoSetupWrapper{}}
}

func (io DirectIODisabler) Configure(path string) error {
	loopbackDev, err := io.LoSetup.FindAssociatedLoopDevice(path)
	if err != nil {
		return err
	}

	return io.LoSetup.DisableDirectIO(loopbackDev)
}

type NoopDirectIO struct{}

func NewNoopDirectIO() *NoopDirectIO {
	return &NoopDirectIO{}
}

func (io NoopDirectIO) Configure(path string) error {
	return nil
}

package loopback

//go:generate counterfeiter . LoSetup
type LoSetup interface {
	FindAssociatedLoopDevice(filePath string) (string, error)
	EnableDirectIO(loopdevPath string) error
}

type DirectIO struct {
	LoSetup LoSetup
}

func NewDirectIO() *DirectIO {
	return &DirectIO{LoSetup: &LoSetupWrapper{}}
}

func (io DirectIO) EnableDirectIO(path string) error {
	loopbackDev, err := io.LoSetup.FindAssociatedLoopDevice(path)
	if err != nil {
		return err
	}

	return io.LoSetup.EnableDirectIO(loopbackDev)
}

type NoopDirectIO struct{}

func NewNoopDirectIO() *NoopDirectIO {
	return &NoopDirectIO{}
}

func (io NoopDirectIO) EnableDirectIO(path string) error {
	return nil
}

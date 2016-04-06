package kawasaki

import "github.com/pivotal-golang/lager"

//go:generate counterfeiter . DnsResolvConfigurer
type DnsResolvConfigurer interface {
	Configure(log lager.Logger) error
}

type HookActioner struct {
	Configurer          Configurer
	DnsResolvConfigurer DnsResolvConfigurer
}

func (h *HookActioner) Run(logger lager.Logger, action string, cfg NetworkConfig, nsPath string) error {
	if action == "create" {
		if err := h.DnsResolvConfigurer.Configure(logger); err != nil {
			return err
		}

		return h.Configurer.Apply(logger, cfg, nsPath)
	}

	return h.Configurer.Destroy(logger, cfg)
}

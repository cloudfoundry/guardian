package bundlerules

import (
	"fmt"

	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

//counterfeiter:generate . Sysctl
type Sysctl interface {
	Get(key string) (uint32, error)
}

type KernelParameter struct {
	sysctl Sysctl
	key    string
	value  uint32
}

func NewKernelParameter(sysctl Sysctl, key string, value uint32) *KernelParameter {
	return &KernelParameter{
		sysctl: sysctl,
		key:    key,
		value:  value,
	}
}

func (r *KernelParameter) Apply(bndl goci.Bndl, spec spec.DesiredContainerSpec) (goci.Bndl, error) {
	var err error
	value := r.value
	if value == 0 {
		value, err = r.sysctl.Get(r.key)
		if err != nil {
			return goci.Bndl{}, fmt.Errorf("failed to retrieve kernel parameter %q: %w", r.key, err)
		}
	}

	if bndl.Spec.Linux.Sysctl == nil {
		bndl.Spec.Linux.Sysctl = make(map[string]string)
	}

	bndl.Spec.Linux.Sysctl[r.key] = fmt.Sprintf("%d", value)
	return bndl, nil
}

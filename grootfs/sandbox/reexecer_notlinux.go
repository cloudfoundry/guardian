//go:build !linux
// +build !linux

package sandbox // import "code.cloudfoundry.org/grootfs/sandbox"

import (
	"errors"
	"os"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager/v3"
)

func Register(commandName string, action func(logger lager.Logger, extraFiles []*os.File, args ...string) error) {
}

func (r *reexecer) Reexec(commandName string, spec groot.ReexecSpec) ([]byte, error) {
	return []byte{}, errors.New("Not implemented on non-linux platforms")
}

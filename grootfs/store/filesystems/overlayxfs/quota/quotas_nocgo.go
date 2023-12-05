//go:build !cgo
// +build !cgo

package quota

import (
	"code.cloudfoundry.org/lager/v3"
	"github.com/pkg/errors"
)

func Get(logger lager.Logger, path string) (Quota, error) {
	logger.Fatal("running-without-cgo-support", errors.New("can't run without cgo support"))
	return Quota{}, nil
}

func Set(logger lager.Logger, projectID uint32, path string, quotaSize uint64) error {
	logger.Fatal("running-without-cgo-support", errors.New("can't run without cgo support"))
	return nil
}

func GetProjectID(logger lager.Logger, path string) (uint32, error) {
	logger.Fatal("running-without-cgo-support", errors.New("can't run without cgo support"))
	return 0, nil
}

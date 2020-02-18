package sysctl

import (
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
)

type Sysctl struct{}

func New() *Sysctl {
	return &Sysctl{}
}

func (s *Sysctl) Get(key string) (uint32, error) {
	path := filepath.Join("/proc/sys", strings.ReplaceAll(key, ".", "/"))

	stringValue, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, err
	}

	trimmedStringValue := strings.TrimSpace(string(stringValue))

	value, err := strconv.ParseUint(trimmedStringValue, 10, 32)
	if err != nil {
		return 0, err
	}

	return uint32(value), nil
}

package sysctl

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Sysctl struct{}

func New() *Sysctl {
	return &Sysctl{}
}

func (s *Sysctl) Get(key string) (uint32, error) {
	stringValue, err := s.GetString(key)
	if err != nil {
		return 0, err
	}

	value, err := strconv.ParseUint(stringValue, 10, 32)
	if err != nil {
		return 0, err
	}

	return uint32(value), nil
}

func (s *Sysctl) GetString(key string) (string, error) {
	path := filepath.Join("/proc/sys", strings.ReplaceAll(key, ".", "/"))

	stringValue, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(stringValue)), nil
}

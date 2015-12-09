package sysinfo

import "github.com/cloudfoundry/gosigar"

type Provider struct {
	depotPath string
}

func NewProvider(depotPath string) Provider {
	return Provider{
		depotPath: depotPath,
	}
}

func (provider Provider) TotalMemory() (uint64, error) {
	mem := sigar.Mem{}

	err := mem.Get()
	if err != nil {
		return 0, err
	}

	return mem.Total, nil
}

func (provider Provider) TotalDisk() (uint64, error) {
	disk := sigar.FileSystemUsage{}

	err := disk.Get(provider.depotPath)
	if err != nil {
		return 0, err
	}

	return fromKBytesToBytes(disk.Total), nil
}

func fromKBytesToBytes(kbytes uint64) uint64 {
	return kbytes * 1024
}

// This file was generated by counterfeiter
package gardenerfakes

import (
	"sync"

	"code.cloudfoundry.org/guardian/gardener"
)

type FakeSysInfoProvider struct {
	TotalMemoryStub        func() (uint64, error)
	totalMemoryMutex       sync.RWMutex
	totalMemoryArgsForCall []struct{}
	totalMemoryReturns     struct {
		result1 uint64
		result2 error
	}
	TotalDiskStub        func() (uint64, error)
	totalDiskMutex       sync.RWMutex
	totalDiskArgsForCall []struct{}
	totalDiskReturns     struct {
		result1 uint64
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeSysInfoProvider) TotalMemory() (uint64, error) {
	fake.totalMemoryMutex.Lock()
	fake.totalMemoryArgsForCall = append(fake.totalMemoryArgsForCall, struct{}{})
	fake.recordInvocation("TotalMemory", []interface{}{})
	fake.totalMemoryMutex.Unlock()
	if fake.TotalMemoryStub != nil {
		return fake.TotalMemoryStub()
	}
	return fake.totalMemoryReturns.result1, fake.totalMemoryReturns.result2
}

func (fake *FakeSysInfoProvider) TotalMemoryCallCount() int {
	fake.totalMemoryMutex.RLock()
	defer fake.totalMemoryMutex.RUnlock()
	return len(fake.totalMemoryArgsForCall)
}

func (fake *FakeSysInfoProvider) TotalMemoryReturns(result1 uint64, result2 error) {
	fake.TotalMemoryStub = nil
	fake.totalMemoryReturns = struct {
		result1 uint64
		result2 error
	}{result1, result2}
}

func (fake *FakeSysInfoProvider) TotalDisk() (uint64, error) {
	fake.totalDiskMutex.Lock()
	fake.totalDiskArgsForCall = append(fake.totalDiskArgsForCall, struct{}{})
	fake.recordInvocation("TotalDisk", []interface{}{})
	fake.totalDiskMutex.Unlock()
	if fake.TotalDiskStub != nil {
		return fake.TotalDiskStub()
	}
	return fake.totalDiskReturns.result1, fake.totalDiskReturns.result2
}

func (fake *FakeSysInfoProvider) TotalDiskCallCount() int {
	fake.totalDiskMutex.RLock()
	defer fake.totalDiskMutex.RUnlock()
	return len(fake.totalDiskArgsForCall)
}

func (fake *FakeSysInfoProvider) TotalDiskReturns(result1 uint64, result2 error) {
	fake.TotalDiskStub = nil
	fake.totalDiskReturns = struct {
		result1 uint64
		result2 error
	}{result1, result2}
}

func (fake *FakeSysInfoProvider) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.totalMemoryMutex.RLock()
	defer fake.totalMemoryMutex.RUnlock()
	fake.totalDiskMutex.RLock()
	defer fake.totalDiskMutex.RUnlock()
	return fake.invocations
}

func (fake *FakeSysInfoProvider) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ gardener.SysInfoProvider = new(FakeSysInfoProvider)

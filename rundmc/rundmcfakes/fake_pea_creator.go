// Code generated by counterfeiter. DO NOT EDIT.
package rundmcfakes

import (
	"sync"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/lager/v3"
)

type FakePeaCreator struct {
	CreatePeaStub        func(lager.Logger, garden.ProcessSpec, garden.ProcessIO, string) (garden.Process, error)
	createPeaMutex       sync.RWMutex
	createPeaArgsForCall []struct {
		arg1 lager.Logger
		arg2 garden.ProcessSpec
		arg3 garden.ProcessIO
		arg4 string
	}
	createPeaReturns struct {
		result1 garden.Process
		result2 error
	}
	createPeaReturnsOnCall map[int]struct {
		result1 garden.Process
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakePeaCreator) CreatePea(arg1 lager.Logger, arg2 garden.ProcessSpec, arg3 garden.ProcessIO, arg4 string) (garden.Process, error) {
	fake.createPeaMutex.Lock()
	ret, specificReturn := fake.createPeaReturnsOnCall[len(fake.createPeaArgsForCall)]
	fake.createPeaArgsForCall = append(fake.createPeaArgsForCall, struct {
		arg1 lager.Logger
		arg2 garden.ProcessSpec
		arg3 garden.ProcessIO
		arg4 string
	}{arg1, arg2, arg3, arg4})
	fake.recordInvocation("CreatePea", []interface{}{arg1, arg2, arg3, arg4})
	fake.createPeaMutex.Unlock()
	if fake.CreatePeaStub != nil {
		return fake.CreatePeaStub(arg1, arg2, arg3, arg4)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	fakeReturns := fake.createPeaReturns
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakePeaCreator) CreatePeaCallCount() int {
	fake.createPeaMutex.RLock()
	defer fake.createPeaMutex.RUnlock()
	return len(fake.createPeaArgsForCall)
}

func (fake *FakePeaCreator) CreatePeaCalls(stub func(lager.Logger, garden.ProcessSpec, garden.ProcessIO, string) (garden.Process, error)) {
	fake.createPeaMutex.Lock()
	defer fake.createPeaMutex.Unlock()
	fake.CreatePeaStub = stub
}

func (fake *FakePeaCreator) CreatePeaArgsForCall(i int) (lager.Logger, garden.ProcessSpec, garden.ProcessIO, string) {
	fake.createPeaMutex.RLock()
	defer fake.createPeaMutex.RUnlock()
	argsForCall := fake.createPeaArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakePeaCreator) CreatePeaReturns(result1 garden.Process, result2 error) {
	fake.createPeaMutex.Lock()
	defer fake.createPeaMutex.Unlock()
	fake.CreatePeaStub = nil
	fake.createPeaReturns = struct {
		result1 garden.Process
		result2 error
	}{result1, result2}
}

func (fake *FakePeaCreator) CreatePeaReturnsOnCall(i int, result1 garden.Process, result2 error) {
	fake.createPeaMutex.Lock()
	defer fake.createPeaMutex.Unlock()
	fake.CreatePeaStub = nil
	if fake.createPeaReturnsOnCall == nil {
		fake.createPeaReturnsOnCall = make(map[int]struct {
			result1 garden.Process
			result2 error
		})
	}
	fake.createPeaReturnsOnCall[i] = struct {
		result1 garden.Process
		result2 error
	}{result1, result2}
}

func (fake *FakePeaCreator) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.createPeaMutex.RLock()
	defer fake.createPeaMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakePeaCreator) recordInvocation(key string, args []interface{}) {
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

var _ rundmc.PeaCreator = new(FakePeaCreator)

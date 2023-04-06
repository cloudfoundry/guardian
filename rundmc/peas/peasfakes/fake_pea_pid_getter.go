// Code generated by counterfeiter. DO NOT EDIT.
package peasfakes

import (
	"sync"

	"code.cloudfoundry.org/guardian/rundmc/peas"
	"code.cloudfoundry.org/lager/v3"
)

type FakePeaPidGetter struct {
	GetPeaPidStub        func(lager.Logger, string, string) (int, error)
	getPeaPidMutex       sync.RWMutex
	getPeaPidArgsForCall []struct {
		arg1 lager.Logger
		arg2 string
		arg3 string
	}
	getPeaPidReturns struct {
		result1 int
		result2 error
	}
	getPeaPidReturnsOnCall map[int]struct {
		result1 int
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakePeaPidGetter) GetPeaPid(arg1 lager.Logger, arg2 string, arg3 string) (int, error) {
	fake.getPeaPidMutex.Lock()
	ret, specificReturn := fake.getPeaPidReturnsOnCall[len(fake.getPeaPidArgsForCall)]
	fake.getPeaPidArgsForCall = append(fake.getPeaPidArgsForCall, struct {
		arg1 lager.Logger
		arg2 string
		arg3 string
	}{arg1, arg2, arg3})
	fake.recordInvocation("GetPeaPid", []interface{}{arg1, arg2, arg3})
	fake.getPeaPidMutex.Unlock()
	if fake.GetPeaPidStub != nil {
		return fake.GetPeaPidStub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	fakeReturns := fake.getPeaPidReturns
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakePeaPidGetter) GetPeaPidCallCount() int {
	fake.getPeaPidMutex.RLock()
	defer fake.getPeaPidMutex.RUnlock()
	return len(fake.getPeaPidArgsForCall)
}

func (fake *FakePeaPidGetter) GetPeaPidCalls(stub func(lager.Logger, string, string) (int, error)) {
	fake.getPeaPidMutex.Lock()
	defer fake.getPeaPidMutex.Unlock()
	fake.GetPeaPidStub = stub
}

func (fake *FakePeaPidGetter) GetPeaPidArgsForCall(i int) (lager.Logger, string, string) {
	fake.getPeaPidMutex.RLock()
	defer fake.getPeaPidMutex.RUnlock()
	argsForCall := fake.getPeaPidArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakePeaPidGetter) GetPeaPidReturns(result1 int, result2 error) {
	fake.getPeaPidMutex.Lock()
	defer fake.getPeaPidMutex.Unlock()
	fake.GetPeaPidStub = nil
	fake.getPeaPidReturns = struct {
		result1 int
		result2 error
	}{result1, result2}
}

func (fake *FakePeaPidGetter) GetPeaPidReturnsOnCall(i int, result1 int, result2 error) {
	fake.getPeaPidMutex.Lock()
	defer fake.getPeaPidMutex.Unlock()
	fake.GetPeaPidStub = nil
	if fake.getPeaPidReturnsOnCall == nil {
		fake.getPeaPidReturnsOnCall = make(map[int]struct {
			result1 int
			result2 error
		})
	}
	fake.getPeaPidReturnsOnCall[i] = struct {
		result1 int
		result2 error
	}{result1, result2}
}

func (fake *FakePeaPidGetter) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.getPeaPidMutex.RLock()
	defer fake.getPeaPidMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakePeaPidGetter) recordInvocation(key string, args []interface{}) {
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

var _ peas.PeaPidGetter = new(FakePeaPidGetter)

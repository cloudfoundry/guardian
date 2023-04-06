// Code generated by counterfeiter. DO NOT EDIT.
package runcontainerdfakes

import (
	"sync"

	"code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	"code.cloudfoundry.org/lager/v3"
)

type FakeRuntime struct {
	DeleteStub        func(lager.Logger, string) error
	deleteMutex       sync.RWMutex
	deleteArgsForCall []struct {
		arg1 lager.Logger
		arg2 string
	}
	deleteReturns struct {
		result1 error
	}
	deleteReturnsOnCall map[int]struct {
		result1 error
	}
	RemoveBundleStub        func(lager.Logger, string) error
	removeBundleMutex       sync.RWMutex
	removeBundleArgsForCall []struct {
		arg1 lager.Logger
		arg2 string
	}
	removeBundleReturns struct {
		result1 error
	}
	removeBundleReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeRuntime) Delete(arg1 lager.Logger, arg2 string) error {
	fake.deleteMutex.Lock()
	ret, specificReturn := fake.deleteReturnsOnCall[len(fake.deleteArgsForCall)]
	fake.deleteArgsForCall = append(fake.deleteArgsForCall, struct {
		arg1 lager.Logger
		arg2 string
	}{arg1, arg2})
	stub := fake.DeleteStub
	fakeReturns := fake.deleteReturns
	fake.recordInvocation("Delete", []interface{}{arg1, arg2})
	fake.deleteMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeRuntime) DeleteCallCount() int {
	fake.deleteMutex.RLock()
	defer fake.deleteMutex.RUnlock()
	return len(fake.deleteArgsForCall)
}

func (fake *FakeRuntime) DeleteCalls(stub func(lager.Logger, string) error) {
	fake.deleteMutex.Lock()
	defer fake.deleteMutex.Unlock()
	fake.DeleteStub = stub
}

func (fake *FakeRuntime) DeleteArgsForCall(i int) (lager.Logger, string) {
	fake.deleteMutex.RLock()
	defer fake.deleteMutex.RUnlock()
	argsForCall := fake.deleteArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeRuntime) DeleteReturns(result1 error) {
	fake.deleteMutex.Lock()
	defer fake.deleteMutex.Unlock()
	fake.DeleteStub = nil
	fake.deleteReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeRuntime) DeleteReturnsOnCall(i int, result1 error) {
	fake.deleteMutex.Lock()
	defer fake.deleteMutex.Unlock()
	fake.DeleteStub = nil
	if fake.deleteReturnsOnCall == nil {
		fake.deleteReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.deleteReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeRuntime) RemoveBundle(arg1 lager.Logger, arg2 string) error {
	fake.removeBundleMutex.Lock()
	ret, specificReturn := fake.removeBundleReturnsOnCall[len(fake.removeBundleArgsForCall)]
	fake.removeBundleArgsForCall = append(fake.removeBundleArgsForCall, struct {
		arg1 lager.Logger
		arg2 string
	}{arg1, arg2})
	stub := fake.RemoveBundleStub
	fakeReturns := fake.removeBundleReturns
	fake.recordInvocation("RemoveBundle", []interface{}{arg1, arg2})
	fake.removeBundleMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeRuntime) RemoveBundleCallCount() int {
	fake.removeBundleMutex.RLock()
	defer fake.removeBundleMutex.RUnlock()
	return len(fake.removeBundleArgsForCall)
}

func (fake *FakeRuntime) RemoveBundleCalls(stub func(lager.Logger, string) error) {
	fake.removeBundleMutex.Lock()
	defer fake.removeBundleMutex.Unlock()
	fake.RemoveBundleStub = stub
}

func (fake *FakeRuntime) RemoveBundleArgsForCall(i int) (lager.Logger, string) {
	fake.removeBundleMutex.RLock()
	defer fake.removeBundleMutex.RUnlock()
	argsForCall := fake.removeBundleArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeRuntime) RemoveBundleReturns(result1 error) {
	fake.removeBundleMutex.Lock()
	defer fake.removeBundleMutex.Unlock()
	fake.RemoveBundleStub = nil
	fake.removeBundleReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeRuntime) RemoveBundleReturnsOnCall(i int, result1 error) {
	fake.removeBundleMutex.Lock()
	defer fake.removeBundleMutex.Unlock()
	fake.RemoveBundleStub = nil
	if fake.removeBundleReturnsOnCall == nil {
		fake.removeBundleReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.removeBundleReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeRuntime) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.deleteMutex.RLock()
	defer fake.deleteMutex.RUnlock()
	fake.removeBundleMutex.RLock()
	defer fake.removeBundleMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeRuntime) recordInvocation(key string, args []interface{}) {
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

var _ runcontainerd.Runtime = new(FakeRuntime)

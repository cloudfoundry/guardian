// Code generated by counterfeiter. DO NOT EDIT.
package peasfakes

import (
	"sync"

	"code.cloudfoundry.org/guardian/rundmc/peas"
)

type FakePrivilegedGetter struct {
	PrivilegedStub        func(string) (bool, error)
	privilegedMutex       sync.RWMutex
	privilegedArgsForCall []struct {
		arg1 string
	}
	privilegedReturns struct {
		result1 bool
		result2 error
	}
	privilegedReturnsOnCall map[int]struct {
		result1 bool
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakePrivilegedGetter) Privileged(arg1 string) (bool, error) {
	fake.privilegedMutex.Lock()
	ret, specificReturn := fake.privilegedReturnsOnCall[len(fake.privilegedArgsForCall)]
	fake.privilegedArgsForCall = append(fake.privilegedArgsForCall, struct {
		arg1 string
	}{arg1})
	fake.recordInvocation("Privileged", []interface{}{arg1})
	fake.privilegedMutex.Unlock()
	if fake.PrivilegedStub != nil {
		return fake.PrivilegedStub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	fakeReturns := fake.privilegedReturns
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakePrivilegedGetter) PrivilegedCallCount() int {
	fake.privilegedMutex.RLock()
	defer fake.privilegedMutex.RUnlock()
	return len(fake.privilegedArgsForCall)
}

func (fake *FakePrivilegedGetter) PrivilegedCalls(stub func(string) (bool, error)) {
	fake.privilegedMutex.Lock()
	defer fake.privilegedMutex.Unlock()
	fake.PrivilegedStub = stub
}

func (fake *FakePrivilegedGetter) PrivilegedArgsForCall(i int) string {
	fake.privilegedMutex.RLock()
	defer fake.privilegedMutex.RUnlock()
	argsForCall := fake.privilegedArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakePrivilegedGetter) PrivilegedReturns(result1 bool, result2 error) {
	fake.privilegedMutex.Lock()
	defer fake.privilegedMutex.Unlock()
	fake.PrivilegedStub = nil
	fake.privilegedReturns = struct {
		result1 bool
		result2 error
	}{result1, result2}
}

func (fake *FakePrivilegedGetter) PrivilegedReturnsOnCall(i int, result1 bool, result2 error) {
	fake.privilegedMutex.Lock()
	defer fake.privilegedMutex.Unlock()
	fake.PrivilegedStub = nil
	if fake.privilegedReturnsOnCall == nil {
		fake.privilegedReturnsOnCall = make(map[int]struct {
			result1 bool
			result2 error
		})
	}
	fake.privilegedReturnsOnCall[i] = struct {
		result1 bool
		result2 error
	}{result1, result2}
}

func (fake *FakePrivilegedGetter) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.privilegedMutex.RLock()
	defer fake.privilegedMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakePrivilegedGetter) recordInvocation(key string, args []interface{}) {
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

var _ peas.PrivilegedGetter = new(FakePrivilegedGetter)

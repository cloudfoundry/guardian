// Code generated by counterfeiter. DO NOT EDIT.
package gardenerfakes

import (
	"sync"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
)

type FakePropertyManager struct {
	AllStub        func(string) (garden.Properties, error)
	allMutex       sync.RWMutex
	allArgsForCall []struct {
		arg1 string
	}
	allReturns struct {
		result1 garden.Properties
		result2 error
	}
	allReturnsOnCall map[int]struct {
		result1 garden.Properties
		result2 error
	}
	DestroyKeySpaceStub        func(string) error
	destroyKeySpaceMutex       sync.RWMutex
	destroyKeySpaceArgsForCall []struct {
		arg1 string
	}
	destroyKeySpaceReturns struct {
		result1 error
	}
	destroyKeySpaceReturnsOnCall map[int]struct {
		result1 error
	}
	GetStub        func(string, string) (string, bool)
	getMutex       sync.RWMutex
	getArgsForCall []struct {
		arg1 string
		arg2 string
	}
	getReturns struct {
		result1 string
		result2 bool
	}
	getReturnsOnCall map[int]struct {
		result1 string
		result2 bool
	}
	MatchesAllStub        func(string, garden.Properties) bool
	matchesAllMutex       sync.RWMutex
	matchesAllArgsForCall []struct {
		arg1 string
		arg2 garden.Properties
	}
	matchesAllReturns struct {
		result1 bool
	}
	matchesAllReturnsOnCall map[int]struct {
		result1 bool
	}
	RemoveStub        func(string, string) error
	removeMutex       sync.RWMutex
	removeArgsForCall []struct {
		arg1 string
		arg2 string
	}
	removeReturns struct {
		result1 error
	}
	removeReturnsOnCall map[int]struct {
		result1 error
	}
	SetStub        func(string, string, string)
	setMutex       sync.RWMutex
	setArgsForCall []struct {
		arg1 string
		arg2 string
		arg3 string
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakePropertyManager) All(arg1 string) (garden.Properties, error) {
	fake.allMutex.Lock()
	ret, specificReturn := fake.allReturnsOnCall[len(fake.allArgsForCall)]
	fake.allArgsForCall = append(fake.allArgsForCall, struct {
		arg1 string
	}{arg1})
	stub := fake.AllStub
	fakeReturns := fake.allReturns
	fake.recordInvocation("All", []interface{}{arg1})
	fake.allMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakePropertyManager) AllCallCount() int {
	fake.allMutex.RLock()
	defer fake.allMutex.RUnlock()
	return len(fake.allArgsForCall)
}

func (fake *FakePropertyManager) AllCalls(stub func(string) (garden.Properties, error)) {
	fake.allMutex.Lock()
	defer fake.allMutex.Unlock()
	fake.AllStub = stub
}

func (fake *FakePropertyManager) AllArgsForCall(i int) string {
	fake.allMutex.RLock()
	defer fake.allMutex.RUnlock()
	argsForCall := fake.allArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakePropertyManager) AllReturns(result1 garden.Properties, result2 error) {
	fake.allMutex.Lock()
	defer fake.allMutex.Unlock()
	fake.AllStub = nil
	fake.allReturns = struct {
		result1 garden.Properties
		result2 error
	}{result1, result2}
}

func (fake *FakePropertyManager) AllReturnsOnCall(i int, result1 garden.Properties, result2 error) {
	fake.allMutex.Lock()
	defer fake.allMutex.Unlock()
	fake.AllStub = nil
	if fake.allReturnsOnCall == nil {
		fake.allReturnsOnCall = make(map[int]struct {
			result1 garden.Properties
			result2 error
		})
	}
	fake.allReturnsOnCall[i] = struct {
		result1 garden.Properties
		result2 error
	}{result1, result2}
}

func (fake *FakePropertyManager) DestroyKeySpace(arg1 string) error {
	fake.destroyKeySpaceMutex.Lock()
	ret, specificReturn := fake.destroyKeySpaceReturnsOnCall[len(fake.destroyKeySpaceArgsForCall)]
	fake.destroyKeySpaceArgsForCall = append(fake.destroyKeySpaceArgsForCall, struct {
		arg1 string
	}{arg1})
	stub := fake.DestroyKeySpaceStub
	fakeReturns := fake.destroyKeySpaceReturns
	fake.recordInvocation("DestroyKeySpace", []interface{}{arg1})
	fake.destroyKeySpaceMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakePropertyManager) DestroyKeySpaceCallCount() int {
	fake.destroyKeySpaceMutex.RLock()
	defer fake.destroyKeySpaceMutex.RUnlock()
	return len(fake.destroyKeySpaceArgsForCall)
}

func (fake *FakePropertyManager) DestroyKeySpaceCalls(stub func(string) error) {
	fake.destroyKeySpaceMutex.Lock()
	defer fake.destroyKeySpaceMutex.Unlock()
	fake.DestroyKeySpaceStub = stub
}

func (fake *FakePropertyManager) DestroyKeySpaceArgsForCall(i int) string {
	fake.destroyKeySpaceMutex.RLock()
	defer fake.destroyKeySpaceMutex.RUnlock()
	argsForCall := fake.destroyKeySpaceArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakePropertyManager) DestroyKeySpaceReturns(result1 error) {
	fake.destroyKeySpaceMutex.Lock()
	defer fake.destroyKeySpaceMutex.Unlock()
	fake.DestroyKeySpaceStub = nil
	fake.destroyKeySpaceReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakePropertyManager) DestroyKeySpaceReturnsOnCall(i int, result1 error) {
	fake.destroyKeySpaceMutex.Lock()
	defer fake.destroyKeySpaceMutex.Unlock()
	fake.DestroyKeySpaceStub = nil
	if fake.destroyKeySpaceReturnsOnCall == nil {
		fake.destroyKeySpaceReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.destroyKeySpaceReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakePropertyManager) Get(arg1 string, arg2 string) (string, bool) {
	fake.getMutex.Lock()
	ret, specificReturn := fake.getReturnsOnCall[len(fake.getArgsForCall)]
	fake.getArgsForCall = append(fake.getArgsForCall, struct {
		arg1 string
		arg2 string
	}{arg1, arg2})
	stub := fake.GetStub
	fakeReturns := fake.getReturns
	fake.recordInvocation("Get", []interface{}{arg1, arg2})
	fake.getMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakePropertyManager) GetCallCount() int {
	fake.getMutex.RLock()
	defer fake.getMutex.RUnlock()
	return len(fake.getArgsForCall)
}

func (fake *FakePropertyManager) GetCalls(stub func(string, string) (string, bool)) {
	fake.getMutex.Lock()
	defer fake.getMutex.Unlock()
	fake.GetStub = stub
}

func (fake *FakePropertyManager) GetArgsForCall(i int) (string, string) {
	fake.getMutex.RLock()
	defer fake.getMutex.RUnlock()
	argsForCall := fake.getArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakePropertyManager) GetReturns(result1 string, result2 bool) {
	fake.getMutex.Lock()
	defer fake.getMutex.Unlock()
	fake.GetStub = nil
	fake.getReturns = struct {
		result1 string
		result2 bool
	}{result1, result2}
}

func (fake *FakePropertyManager) GetReturnsOnCall(i int, result1 string, result2 bool) {
	fake.getMutex.Lock()
	defer fake.getMutex.Unlock()
	fake.GetStub = nil
	if fake.getReturnsOnCall == nil {
		fake.getReturnsOnCall = make(map[int]struct {
			result1 string
			result2 bool
		})
	}
	fake.getReturnsOnCall[i] = struct {
		result1 string
		result2 bool
	}{result1, result2}
}

func (fake *FakePropertyManager) MatchesAll(arg1 string, arg2 garden.Properties) bool {
	fake.matchesAllMutex.Lock()
	ret, specificReturn := fake.matchesAllReturnsOnCall[len(fake.matchesAllArgsForCall)]
	fake.matchesAllArgsForCall = append(fake.matchesAllArgsForCall, struct {
		arg1 string
		arg2 garden.Properties
	}{arg1, arg2})
	stub := fake.MatchesAllStub
	fakeReturns := fake.matchesAllReturns
	fake.recordInvocation("MatchesAll", []interface{}{arg1, arg2})
	fake.matchesAllMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakePropertyManager) MatchesAllCallCount() int {
	fake.matchesAllMutex.RLock()
	defer fake.matchesAllMutex.RUnlock()
	return len(fake.matchesAllArgsForCall)
}

func (fake *FakePropertyManager) MatchesAllCalls(stub func(string, garden.Properties) bool) {
	fake.matchesAllMutex.Lock()
	defer fake.matchesAllMutex.Unlock()
	fake.MatchesAllStub = stub
}

func (fake *FakePropertyManager) MatchesAllArgsForCall(i int) (string, garden.Properties) {
	fake.matchesAllMutex.RLock()
	defer fake.matchesAllMutex.RUnlock()
	argsForCall := fake.matchesAllArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakePropertyManager) MatchesAllReturns(result1 bool) {
	fake.matchesAllMutex.Lock()
	defer fake.matchesAllMutex.Unlock()
	fake.MatchesAllStub = nil
	fake.matchesAllReturns = struct {
		result1 bool
	}{result1}
}

func (fake *FakePropertyManager) MatchesAllReturnsOnCall(i int, result1 bool) {
	fake.matchesAllMutex.Lock()
	defer fake.matchesAllMutex.Unlock()
	fake.MatchesAllStub = nil
	if fake.matchesAllReturnsOnCall == nil {
		fake.matchesAllReturnsOnCall = make(map[int]struct {
			result1 bool
		})
	}
	fake.matchesAllReturnsOnCall[i] = struct {
		result1 bool
	}{result1}
}

func (fake *FakePropertyManager) Remove(arg1 string, arg2 string) error {
	fake.removeMutex.Lock()
	ret, specificReturn := fake.removeReturnsOnCall[len(fake.removeArgsForCall)]
	fake.removeArgsForCall = append(fake.removeArgsForCall, struct {
		arg1 string
		arg2 string
	}{arg1, arg2})
	stub := fake.RemoveStub
	fakeReturns := fake.removeReturns
	fake.recordInvocation("Remove", []interface{}{arg1, arg2})
	fake.removeMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakePropertyManager) RemoveCallCount() int {
	fake.removeMutex.RLock()
	defer fake.removeMutex.RUnlock()
	return len(fake.removeArgsForCall)
}

func (fake *FakePropertyManager) RemoveCalls(stub func(string, string) error) {
	fake.removeMutex.Lock()
	defer fake.removeMutex.Unlock()
	fake.RemoveStub = stub
}

func (fake *FakePropertyManager) RemoveArgsForCall(i int) (string, string) {
	fake.removeMutex.RLock()
	defer fake.removeMutex.RUnlock()
	argsForCall := fake.removeArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakePropertyManager) RemoveReturns(result1 error) {
	fake.removeMutex.Lock()
	defer fake.removeMutex.Unlock()
	fake.RemoveStub = nil
	fake.removeReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakePropertyManager) RemoveReturnsOnCall(i int, result1 error) {
	fake.removeMutex.Lock()
	defer fake.removeMutex.Unlock()
	fake.RemoveStub = nil
	if fake.removeReturnsOnCall == nil {
		fake.removeReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.removeReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakePropertyManager) Set(arg1 string, arg2 string, arg3 string) {
	fake.setMutex.Lock()
	fake.setArgsForCall = append(fake.setArgsForCall, struct {
		arg1 string
		arg2 string
		arg3 string
	}{arg1, arg2, arg3})
	stub := fake.SetStub
	fake.recordInvocation("Set", []interface{}{arg1, arg2, arg3})
	fake.setMutex.Unlock()
	if stub != nil {
		fake.SetStub(arg1, arg2, arg3)
	}
}

func (fake *FakePropertyManager) SetCallCount() int {
	fake.setMutex.RLock()
	defer fake.setMutex.RUnlock()
	return len(fake.setArgsForCall)
}

func (fake *FakePropertyManager) SetCalls(stub func(string, string, string)) {
	fake.setMutex.Lock()
	defer fake.setMutex.Unlock()
	fake.SetStub = stub
}

func (fake *FakePropertyManager) SetArgsForCall(i int) (string, string, string) {
	fake.setMutex.RLock()
	defer fake.setMutex.RUnlock()
	argsForCall := fake.setArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakePropertyManager) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.allMutex.RLock()
	defer fake.allMutex.RUnlock()
	fake.destroyKeySpaceMutex.RLock()
	defer fake.destroyKeySpaceMutex.RUnlock()
	fake.getMutex.RLock()
	defer fake.getMutex.RUnlock()
	fake.matchesAllMutex.RLock()
	defer fake.matchesAllMutex.RUnlock()
	fake.removeMutex.RLock()
	defer fake.removeMutex.RUnlock()
	fake.setMutex.RLock()
	defer fake.setMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakePropertyManager) recordInvocation(key string, args []interface{}) {
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

var _ gardener.PropertyManager = new(FakePropertyManager)

// This file was generated by counterfeiter
package subnetsfakes

import (
	"net"
	"sync"

	"code.cloudfoundry.org/guardian/kawasaki/subnets"
)

type FakeIPSelector struct {
	SelectIPStub        func(subnet *net.IPNet, existing []net.IP) (net.IP, error)
	selectIPMutex       sync.RWMutex
	selectIPArgsForCall []struct {
		subnet   *net.IPNet
		existing []net.IP
	}
	selectIPReturns struct {
		result1 net.IP
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeIPSelector) SelectIP(subnet *net.IPNet, existing []net.IP) (net.IP, error) {
	var existingCopy []net.IP
	if existing != nil {
		existingCopy = make([]net.IP, len(existing))
		copy(existingCopy, existing)
	}
	fake.selectIPMutex.Lock()
	fake.selectIPArgsForCall = append(fake.selectIPArgsForCall, struct {
		subnet   *net.IPNet
		existing []net.IP
	}{subnet, existingCopy})
	fake.recordInvocation("SelectIP", []interface{}{subnet, existingCopy})
	fake.selectIPMutex.Unlock()
	if fake.SelectIPStub != nil {
		return fake.SelectIPStub(subnet, existing)
	}
	return fake.selectIPReturns.result1, fake.selectIPReturns.result2
}

func (fake *FakeIPSelector) SelectIPCallCount() int {
	fake.selectIPMutex.RLock()
	defer fake.selectIPMutex.RUnlock()
	return len(fake.selectIPArgsForCall)
}

func (fake *FakeIPSelector) SelectIPArgsForCall(i int) (*net.IPNet, []net.IP) {
	fake.selectIPMutex.RLock()
	defer fake.selectIPMutex.RUnlock()
	return fake.selectIPArgsForCall[i].subnet, fake.selectIPArgsForCall[i].existing
}

func (fake *FakeIPSelector) SelectIPReturns(result1 net.IP, result2 error) {
	fake.SelectIPStub = nil
	fake.selectIPReturns = struct {
		result1 net.IP
		result2 error
	}{result1, result2}
}

func (fake *FakeIPSelector) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.selectIPMutex.RLock()
	defer fake.selectIPMutex.RUnlock()
	return fake.invocations
}

func (fake *FakeIPSelector) recordInvocation(key string, args []interface{}) {
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

var _ subnets.IPSelector = new(FakeIPSelector)

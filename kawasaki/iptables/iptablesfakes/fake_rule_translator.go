// Code generated by counterfeiter. DO NOT EDIT.
package iptablesfakes

import (
	"sync"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/kawasaki/iptables"
)

type FakeRuleTranslator struct {
	TranslateRuleStub        func(string, garden.NetOutRule) ([]iptables.Rule, error)
	translateRuleMutex       sync.RWMutex
	translateRuleArgsForCall []struct {
		arg1 string
		arg2 garden.NetOutRule
	}
	translateRuleReturns struct {
		result1 []iptables.Rule
		result2 error
	}
	translateRuleReturnsOnCall map[int]struct {
		result1 []iptables.Rule
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeRuleTranslator) TranslateRule(arg1 string, arg2 garden.NetOutRule) ([]iptables.Rule, error) {
	fake.translateRuleMutex.Lock()
	ret, specificReturn := fake.translateRuleReturnsOnCall[len(fake.translateRuleArgsForCall)]
	fake.translateRuleArgsForCall = append(fake.translateRuleArgsForCall, struct {
		arg1 string
		arg2 garden.NetOutRule
	}{arg1, arg2})
	stub := fake.TranslateRuleStub
	fakeReturns := fake.translateRuleReturns
	fake.recordInvocation("TranslateRule", []interface{}{arg1, arg2})
	fake.translateRuleMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeRuleTranslator) TranslateRuleCallCount() int {
	fake.translateRuleMutex.RLock()
	defer fake.translateRuleMutex.RUnlock()
	return len(fake.translateRuleArgsForCall)
}

func (fake *FakeRuleTranslator) TranslateRuleCalls(stub func(string, garden.NetOutRule) ([]iptables.Rule, error)) {
	fake.translateRuleMutex.Lock()
	defer fake.translateRuleMutex.Unlock()
	fake.TranslateRuleStub = stub
}

func (fake *FakeRuleTranslator) TranslateRuleArgsForCall(i int) (string, garden.NetOutRule) {
	fake.translateRuleMutex.RLock()
	defer fake.translateRuleMutex.RUnlock()
	argsForCall := fake.translateRuleArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeRuleTranslator) TranslateRuleReturns(result1 []iptables.Rule, result2 error) {
	fake.translateRuleMutex.Lock()
	defer fake.translateRuleMutex.Unlock()
	fake.TranslateRuleStub = nil
	fake.translateRuleReturns = struct {
		result1 []iptables.Rule
		result2 error
	}{result1, result2}
}

func (fake *FakeRuleTranslator) TranslateRuleReturnsOnCall(i int, result1 []iptables.Rule, result2 error) {
	fake.translateRuleMutex.Lock()
	defer fake.translateRuleMutex.Unlock()
	fake.TranslateRuleStub = nil
	if fake.translateRuleReturnsOnCall == nil {
		fake.translateRuleReturnsOnCall = make(map[int]struct {
			result1 []iptables.Rule
			result2 error
		})
	}
	fake.translateRuleReturnsOnCall[i] = struct {
		result1 []iptables.Rule
		result2 error
	}{result1, result2}
}

func (fake *FakeRuleTranslator) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.translateRuleMutex.RLock()
	defer fake.translateRuleMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeRuleTranslator) recordInvocation(key string, args []interface{}) {
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

var _ iptables.RuleTranslator = new(FakeRuleTranslator)

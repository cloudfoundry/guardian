package iptables_test

import (
	"errors"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/kawasaki/iptables"
	fakes "code.cloudfoundry.org/guardian/kawasaki/iptables/iptablesfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FirewallOpenner", func() {
	var (
		logger                 lager.Logger
		fakeRuleTranslator     *fakes.FakeRuleTranslator
		fakeIPTablesController *fakes.FakeIPTables
		opener                 *iptables.FirewallOpener
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeRuleTranslator = new(fakes.FakeRuleTranslator)
		fakeRuleTranslator.TranslateRuleReturns([]iptables.Rule{
			iptables.SingleFilterRule{},
		}, nil)

		fakeIPTablesController = new(fakes.FakeIPTables)
		fakeIPTablesController.InstanceChainStub = func(chain string) string {
			return "prefix-" + chain
		}

		opener = iptables.NewFirewallOpener(
			fakeRuleTranslator, fakeIPTablesController,
		)
	})

	Describe("Open", func() {
		It("builds the correct rules", func() {
			rule := garden.NetOutRule{Protocol: garden.ProtocolUDP}
			Expect(opener.Open(logger, "foo-bar-baz", "some-handle", rule)).To(Succeed())
			actualHandle, actualRule := fakeRuleTranslator.TranslateRuleArgsForCall(0)
			Expect(actualHandle).To(Equal("some-handle"))
			Expect(actualRule).To(Equal(rule))
		})

		Context("when building the rules fails", func() {
			BeforeEach(func() {
				fakeRuleTranslator.TranslateRuleReturns(nil, errors.New("failed to build rules"))
			})

			It("returns the error", func() {
				Expect(opener.Open(logger, "foo-bar-baz", "some-handle", garden.NetOutRule{})).To(MatchError("failed to build rules"))
			})
		})

		It("prepends the built rules", func() {
			rules := []iptables.Rule{
				iptables.SingleFilterRule{
					Protocol: garden.ProtocolTCP,
				},
				iptables.SingleFilterRule{
					Protocol: garden.ProtocolUDP,
				},
			}
			fakeRuleTranslator.TranslateRuleReturns(rules, nil)

			Expect(opener.Open(logger, "foo-bar-baz", "some-handle", garden.NetOutRule{})).To(Succeed())

			Expect(fakeIPTablesController.PrependRuleCallCount()).To(Equal(2))
			_, ruleA := fakeIPTablesController.PrependRuleArgsForCall(0)
			Expect(ruleA).To(Equal(rules[0]))
			_, ruleB := fakeIPTablesController.PrependRuleArgsForCall(1)
			Expect(ruleB).To(Equal(rules[1]))
		})

		It("uses the correct chain name", func() {
			Expect(opener.Open(logger, "foo-bar-baz", "some-handle", garden.NetOutRule{})).To(Succeed())

			Expect(fakeIPTablesController.PrependRuleCallCount()).To(Equal(1))
			chainName, _ := fakeIPTablesController.PrependRuleArgsForCall(0)
			Expect(chainName).To(Equal("prefix-foo-bar-baz"))
		})

		Context("when prepending a rule fails", func() {
			BeforeEach(func() {
				fakeIPTablesController.PrependRuleReturns(errors.New("i-lost-my-banana"))
			})

			It("returns the error", func() {
				Expect(opener.Open(logger, "foo-bar-baz", "some-handle", garden.NetOutRule{})).To(MatchError("i-lost-my-banana"))
			})
		})
	})

	Describe("BulkOpen", func() {
		var rules []garden.NetOutRule

		BeforeEach(func() {
			rules = []garden.NetOutRule{
				garden.NetOutRule{Protocol: garden.ProtocolUDP},
				garden.NetOutRule{Protocol: garden.ProtocolTCP},
			}
		})

		It("translates the rules", func() {
			Expect(opener.BulkOpen(logger, "foo-bar-baz", "some-handle", rules)).To(Succeed())
			allRules := []garden.NetOutRule{}
			for i := 0; i < fakeRuleTranslator.TranslateRuleCallCount(); i++ {
				handle, rule := fakeRuleTranslator.TranslateRuleArgsForCall(i)
				Expect(handle).To(Equal("some-handle"))
				allRules = append(allRules, rule)
			}
			Expect(allRules).To(ConsistOf(rules))
		})

		Context("when translating a rule fails", func() {
			BeforeEach(func() {
				fakeRuleTranslator.TranslateRuleReturns(nil, errors.New("failed to build rules"))
			})

			It("returns the error", func() {
				Expect(opener.BulkOpen(logger, "foo-bar-baz", "some-handle", rules)).To(MatchError("failed to build rules"))
			})
		})

		It("prepends them in bulk", func() {
			iptablesRules := [][]iptables.Rule{
				{
					iptables.SingleFilterRule{
						Protocol: garden.ProtocolTCP,
					},
					iptables.SingleFilterRule{
						Protocol: garden.ProtocolUDP,
					},
				},
				{
					iptables.SingleFilterRule{
						Protocol: garden.ProtocolICMP,
					},
					iptables.SingleFilterRule{
						Protocol: garden.ProtocolAll,
					},
				},
			}

			i := 0
			fakeRuleTranslator.TranslateRuleStub = func(_ string, gardenRule garden.NetOutRule) ([]iptables.Rule, error) {
				defer func() { i++ }()
				return iptablesRules[i], nil
			}

			Expect(opener.BulkOpen(logger, "foo-bar-baz", "some-handle", rules)).To(Succeed())

			Expect(fakeIPTablesController.BulkPrependRulesCallCount()).To(Equal(1))
			_, appendedIPTablesRules := fakeIPTablesController.BulkPrependRulesArgsForCall(0)
			Expect(appendedIPTablesRules).To(HaveLen(4))
			Expect(appendedIPTablesRules[0]).To(Equal(iptablesRules[0][0]))
			Expect(appendedIPTablesRules[1]).To(Equal(iptablesRules[0][1]))
			Expect(appendedIPTablesRules[2]).To(Equal(iptablesRules[1][0]))
			Expect(appendedIPTablesRules[3]).To(Equal(iptablesRules[1][1]))
		})

		It("prepends to the correct chain name", func() {
			Expect(opener.BulkOpen(logger, "foo-bar-baz", "some-handle", rules)).To(Succeed())
			Expect(fakeIPTablesController.BulkPrependRulesCallCount()).To(Equal(1))
			chainName, _ := fakeIPTablesController.BulkPrependRulesArgsForCall(0)
			Expect(chainName).To(Equal("prefix-foo-bar-baz"))
		})

		Context("when prepending the rules fails", func() {
			BeforeEach(func() {
				fakeIPTablesController.BulkPrependRulesReturns(errors.New("i-lost-my-banana"))
			})

			It("returns the error", func() {
				Expect(opener.BulkOpen(logger, "foo-bar-baz", "some-handle", rules)).To(MatchError("i-lost-my-banana"))
			})
		})
	})
})

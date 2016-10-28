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
			Expect(opener.Open(logger, "foo-bar-baz", rule)).To(Succeed())
			Expect(fakeRuleTranslator.TranslateRuleArgsForCall(0)).To(Equal(rule))
		})

		Context("when building the rules fails", func() {
			BeforeEach(func() {
				fakeRuleTranslator.TranslateRuleReturns(nil, errors.New("failed to build rules"))
			})

			It("returns the error", func() {
				Expect(opener.Open(logger, "foo-bar-baz", garden.NetOutRule{})).To(MatchError("failed to build rules"))
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

			Expect(opener.Open(logger, "foo-bar-baz", garden.NetOutRule{})).To(Succeed())

			Expect(fakeIPTablesController.PrependRuleCallCount()).To(Equal(2))
			_, ruleA := fakeIPTablesController.PrependRuleArgsForCall(0)
			Expect(ruleA).To(Equal(rules[0]))
			_, ruleB := fakeIPTablesController.PrependRuleArgsForCall(1)
			Expect(ruleB).To(Equal(rules[1]))
		})

		It("uses the correct chain name", func() {
			Expect(opener.Open(logger, "foo-bar-baz", garden.NetOutRule{})).To(Succeed())

			Expect(fakeIPTablesController.PrependRuleCallCount()).To(Equal(1))
			chainName, _ := fakeIPTablesController.PrependRuleArgsForCall(0)
			Expect(chainName).To(Equal("prefix-foo-bar-baz"))
		})

		Context("when prepending a rule fails", func() {
			BeforeEach(func() {
				fakeIPTablesController.PrependRuleReturns(errors.New("i-lost-my-banana"))
			})

			It("returns the error", func() {
				Expect(opener.Open(logger, "foo-bar-baz", garden.NetOutRule{})).To(MatchError("i-lost-my-banana"))
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

		It("builds the correct rules", func() {
			Expect(opener.BulkOpen(logger, "foo-bar-baz", rules)).To(Succeed())
			Expect(fakeRuleTranslator.TranslateRuleArgsForCall(0)).To(Equal(rules[0]))
			Expect(fakeRuleTranslator.TranslateRuleArgsForCall(1)).To(Equal(rules[1]))
		})

		Context("when building the rules fails", func() {
			BeforeEach(func() {
				fakeRuleTranslator.TranslateRuleReturns(nil, errors.New("failed to build rules"))
			})

			It("returns the error", func() {
				Expect(opener.BulkOpen(logger, "foo-bar-baz", rules)).To(MatchError("failed to build rules"))
			})
		})

		It("prepends the built rules", func() {
			iptablesRules := []iptables.Rule{
				iptables.SingleFilterRule{
					Protocol: garden.ProtocolTCP,
				},
				iptables.SingleFilterRule{
					Protocol: garden.ProtocolUDP,
				},
			}
			fakeRuleTranslator.TranslateRuleReturns(iptablesRules, nil)

			Expect(opener.BulkOpen(logger, "foo-bar-baz", rules)).To(Succeed())

			Expect(fakeIPTablesController.PrependRuleCallCount()).To(Equal(4))
			_, ruleA := fakeIPTablesController.PrependRuleArgsForCall(0)
			Expect(ruleA).To(Equal(iptablesRules[0]))
			_, ruleB := fakeIPTablesController.PrependRuleArgsForCall(1)
			Expect(ruleB).To(Equal(iptablesRules[1]))
			_, ruleA = fakeIPTablesController.PrependRuleArgsForCall(2)
			Expect(ruleA).To(Equal(iptablesRules[0]))
			_, ruleB = fakeIPTablesController.PrependRuleArgsForCall(3)
			Expect(ruleB).To(Equal(iptablesRules[1]))
		})

		It("uses the correct chain name", func() {
			Expect(opener.BulkOpen(logger, "foo-bar-baz", rules)).To(Succeed())

			Expect(fakeIPTablesController.PrependRuleCallCount()).To(Equal(2))
			chainName, _ := fakeIPTablesController.PrependRuleArgsForCall(0)
			Expect(chainName).To(Equal("prefix-foo-bar-baz"))
			chainName, _ = fakeIPTablesController.PrependRuleArgsForCall(1)
			Expect(chainName).To(Equal("prefix-foo-bar-baz"))
		})

		Context("when prepending a rule fails", func() {
			BeforeEach(func() {
				fakeIPTablesController.PrependRuleReturns(errors.New("i-lost-my-banana"))
			})

			It("returns the error", func() {
				Expect(opener.BulkOpen(logger, "foo-bar-baz", rules)).To(MatchError("i-lost-my-banana"))
			})
		})
	})
})

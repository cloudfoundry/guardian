package iptables_test

import (
	"net"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/kawasaki/iptables"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("RuleBuilder", func() {
	var translator iptables.RuleTranslator

	BeforeEach(func() {
		translator = iptables.NewRuleTranslator()
	})

	It("applies the default rule", func() {
		iptablesRules, err := translator.TranslateRule("some-handle", garden.NetOutRule{})
		Expect(err).NotTo(HaveOccurred())
		Expect(iptablesRules).To(HaveLen(1))
		Expect(iptablesRules[0]).To(Equal(iptables.SingleFilterRule{
			Handle: "some-handle",
		}))
	})

	Context("when a port range is specified for ProtocolALL", func() {
		It("returns a nice error message", func() {
			_, err := translator.TranslateRule("some-handle", garden.NetOutRule{
				Protocol: garden.ProtocolAll,
				Ports:    []garden.PortRange{{Start: 1, End: 5}},
			})
			Expect(err).To(MatchError("Ports cannot be specified for Protocol ALL"))
		})
	})

	Context("when an invaild protocol is specified", func() {
		It("returns an error", func() {
			_, err := translator.TranslateRule("some-handle", garden.NetOutRule{
				Protocol: garden.Protocol(52),
			})
			Expect(err).To(MatchError("invalid protocol: 52"))
		})
	})

	It("sets the protocol in the rule", func() {
		iptablesRules, err := translator.TranslateRule("some-handle", garden.NetOutRule{
			Protocol: garden.ProtocolTCP,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(iptablesRules).To(HaveLen(1))
		Expect(iptablesRules[0]).To(Equal(iptables.SingleFilterRule{
			Handle:   "some-handle",
			Protocol: garden.ProtocolTCP,
		}))
	})

	It("sets the IMCP control in the rule", func() {
		icmpControl := &garden.ICMPControl{
			Type: garden.ICMPType(1),
		}

		iptablesRules, err := translator.TranslateRule("some-handle", garden.NetOutRule{
			ICMPs: icmpControl,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(iptablesRules).To(HaveLen(1))
		Expect(iptablesRules[0]).To(Equal(iptables.SingleFilterRule{
			Handle: "some-handle",
			ICMPs:  icmpControl,
		}))
	})

	Describe("Log", func() {
		It("sets the log flag to the rule", func() {
			iptablesRules, err := translator.TranslateRule("some-handle", garden.NetOutRule{
				Log: true,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(iptablesRules).To(HaveLen(1))
			Expect(iptablesRules[0]).To(Equal(iptables.SingleFilterRule{
				Handle: "some-handle",
				Log:    true,
			}))
		})
	})

	DescribeTable("networks and ports",
		func(netOutRule garden.NetOutRule, expectedIptablesRules []iptables.SingleFilterRule) {
			iptablesRules, err := translator.TranslateRule("some-handle", netOutRule)
			Expect(err).NotTo(HaveOccurred())

			Expect(iptablesRules).To(HaveLen(len(expectedIptablesRules)))
			for i := 0; i < len(expectedIptablesRules); i++ {
				Expect(iptablesRules[i]).To(Equal(expectedIptablesRules[i]))
			}
		},
		Entry("with a single destination IP specified",
			garden.NetOutRule{Networks: []garden.IPRange{{Start: net.ParseIP("1.2.3.4")}}},
			[]iptables.SingleFilterRule{
				{Handle: "some-handle", Networks: &garden.IPRange{Start: net.ParseIP("1.2.3.4")}},
			},
		),
		Entry("with multiple destination networks specified",
			garden.NetOutRule{Networks: []garden.IPRange{
				{Start: net.ParseIP("1.2.3.4")},
				{Start: net.ParseIP("2.2.3.4"), End: net.ParseIP("2.2.3.9")},
			}},
			[]iptables.SingleFilterRule{
				{Handle: "some-handle", Networks: &garden.IPRange{Start: net.ParseIP("1.2.3.4")}},
				{Handle: "some-handle", Networks: &garden.IPRange{Start: net.ParseIP("2.2.3.4"), End: net.ParseIP("2.2.3.9")}},
			},
		),
		Entry("with a single port specified",
			garden.NetOutRule{
				Protocol: garden.ProtocolTCP,
				Ports: []garden.PortRange{
					garden.PortRangeFromPort(22),
				},
			},
			[]iptables.SingleFilterRule{
				{Handle: "some-handle", Protocol: garden.ProtocolTCP, Ports: &garden.PortRange{Start: 22, End: 22}},
			},
		),
		Entry("with multiple ports specified",
			garden.NetOutRule{
				Protocol: garden.ProtocolTCP,
				Ports: []garden.PortRange{
					garden.PortRangeFromPort(22),
					garden.PortRange{Start: 1000, End: 10000},
				},
			},
			[]iptables.SingleFilterRule{
				{Handle: "some-handle", Protocol: garden.ProtocolTCP, Ports: &garden.PortRange{Start: 22, End: 22}},
				{Handle: "some-handle", Protocol: garden.ProtocolTCP, Ports: &garden.PortRange{Start: 1000, End: 10000}},
			},
		),
		Entry("with both networks and ports specified",
			garden.NetOutRule{
				Protocol: garden.ProtocolTCP,
				Networks: []garden.IPRange{
					{Start: net.ParseIP("1.2.3.4")},
					{Start: net.ParseIP("2.2.3.4"), End: net.ParseIP("2.2.3.9")},
				},
				Ports: []garden.PortRange{
					garden.PortRangeFromPort(22),
					garden.PortRange{Start: 1000, End: 10000},
				},
			},
			[]iptables.SingleFilterRule{
				{
					Handle:   "some-handle",
					Protocol: garden.ProtocolTCP,
					Networks: &garden.IPRange{Start: net.ParseIP("1.2.3.4")},
					Ports:    &garden.PortRange{Start: 22, End: 22},
				},
				{
					Handle:   "some-handle",
					Protocol: garden.ProtocolTCP,
					Networks: &garden.IPRange{Start: net.ParseIP("2.2.3.4"), End: net.ParseIP("2.2.3.9")},
					Ports:    &garden.PortRange{Start: 22, End: 22},
				},
				{
					Handle:   "some-handle",
					Protocol: garden.ProtocolTCP,
					Networks: &garden.IPRange{Start: net.ParseIP("1.2.3.4")},
					Ports:    &garden.PortRange{Start: 1000, End: 10000},
				},
				{
					Handle:   "some-handle",
					Protocol: garden.ProtocolTCP,
					Networks: &garden.IPRange{Start: net.ParseIP("2.2.3.4"), End: net.ParseIP("2.2.3.9")},
					Ports:    &garden.PortRange{Start: 1000, End: 10000},
				},
			},
		),
	)
})

package vars_test

import (
	"net"

	"code.cloudfoundry.org/guardian/pkg/vars"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Vars", func() {
	Describe("StringList", func() {
		Describe("when set is called", func() {

			var sl *vars.StringList

			BeforeEach(func() {
				sl = &vars.StringList{}
				Expect(sl.Set("foo")).To(Succeed())
				Expect(sl.Set("bar")).To(Succeed())
			})

			It("adds the value to the list", func() {
				Expect(sl.List).To(ConsistOf("foo", "bar"))
			})

			It("stringifies with commas", func() {
				Expect(sl.String()).To(Equal("foo, bar"))
			})

			It("returns the list from Get()", func() {
				Expect(sl.Get()).To(Equal([]string{"foo", "bar"}))
			})
		})
	})

	Describe("IPList", func() {
		Describe("when set is called", func() {

			var ipl *vars.IPList
			var ips []net.IP

			BeforeEach(func() {
				ips = nil
				ipl = &vars.IPList{List: &ips}
				Expect(ipl.Set("1.2.3.4")).To(Succeed())
				Expect(ipl.Set("::1")).To(Succeed())
			})

			It("adds the value to the list", func() {
				Expect(ips).To(ConsistOf(net.ParseIP("1.2.3.4"), net.ParseIP("::1")))
			})

			It("stringifies with commas", func() {
				Expect(ipl.String()).To(Equal("1.2.3.4, ::1"))
			})

			It("rejects invalid addresses", func() {
				Expect(ipl.Set("www.example.com")).To(MatchError("'www.example.com' is not a valid IP address"))
			})
		})
	})
})

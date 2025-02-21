package guardiancmd_test

import (
	"code.cloudfoundry.org/guardian/guardiancmd"
	"github.com/jessevdk/go-flags"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DeviceCgroupRuleFlag", func() {
	var cmd *guardiancmd.CommonCommand

	Describe("Unmarshal", func() {
		BeforeEach(func() {
			cmd = &guardiancmd.CommonCommand{}
		})

		It("parses as LinuxDeviceCgroup", func() {
			parser := flags.NewParser(cmd, flags.Default)
			_, err := parser.ParseArgs([]string{"--device-cgroup-rule", "c 1:3 mr"})
			Expect(err).NotTo(HaveOccurred())
			Expect(cmd.Containers.DeviceCgroupRules).To(Equal([]guardiancmd.DeviceCgroupRuleFlag{
				{
					Access: "mr",
					Type:   "c",
					Major:  intRef(1),
					Minor:  intRef(3),
					Allow:  true,
				},
			}))

			_, err = parser.ParseArgs([]string{"--device-cgroup-rule", "b 7:* rwm"})
			Expect(err).NotTo(HaveOccurred())
			Expect(cmd.Containers.DeviceCgroupRules).To(Equal([]guardiancmd.DeviceCgroupRuleFlag{
				{
					Access: "rwm",
					Type:   "b",
					Major:  intRef(7),
					Minor:  intRef(-1),
					Allow:  true,
				},
			}))
		})
	})
})

func intRef(i int64) *int64 {
	return &i
}

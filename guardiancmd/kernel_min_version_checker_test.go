package guardiancmd_test

import (
	"errors"

	"code.cloudfoundry.org/guardian/guardiancmd"
	"code.cloudfoundry.org/guardian/guardiancmd/guardiancmdfakes"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = DescribeTable("Checking min version with kernel 5.4.0-72-generic", func(maj, min, patch int, expectedOutput bool) {
	sysctlGetter := new(guardiancmdfakes.FakeSysctlGetter)
	sysctlGetter.GetStringReturns("5.4.0-72-generic", nil)

	minVerChecker := guardiancmd.NewKernelMinVersionChecker(sysctlGetter)

	ok, err := minVerChecker.CheckVersionIsAtLeast(uint16(maj), uint16(min), uint16(patch))

	Expect(ok).To(Equal(expectedOutput))
	Expect(err).NotTo(HaveOccurred())
},
	Entry("5.4.0", 5, 4, 0, true),
	Entry("5.3.9999", 5, 3, 9999, true),
	Entry("5.4.1", 5, 4, 1, false),
	Entry("0.0.0", 0, 0, 0, true),
	Entry("9.9.9", 9, 9, 9, false),
)

var _ = DescribeTable("Checking kernel versions against 4.8.0", func(kernelVersion string, expectedOutput bool, expectedErr error) {
	sysctlGetter := new(guardiancmdfakes.FakeSysctlGetter)
	sysctlGetter.GetStringReturns(kernelVersion, nil)

	minVerChecker := guardiancmd.NewKernelMinVersionChecker(sysctlGetter)

	ok, err := minVerChecker.CheckVersionIsAtLeast(4, 8, 0)

	Expect(ok).To(Equal(expectedOutput))
	if expectedErr == nil {
		Expect(err).NotTo(HaveOccurred())
	} else {
		Expect(err).To(MatchError(expectedErr))
	}
},
	Entry("5.4.0-72-generic", "5.4.0-72-generic", true, nil),
	Entry("5.4.0+72-generic", "5.4.0+72-generic", true, nil),
	Entry("5.4.89+", "5.4.89+", true, nil),
	Entry("4.18.0-240.22.1.el8_3.x86_64", "4.18.0-240.22.1.el8_3.x86_64", true, nil),
	Entry("2 digit semver with -", "5.4-72-generic", true, nil),
	Entry("1 digit semver with -", "5-72-generic", true, nil),
	Entry("1 digit semver with -", "4-72-generic", false, nil),
	Entry("1 digit semver", "3", false, nil),
	Entry("nonsense", "ubuntu-4.15.2-generic", false, errors.New("Malformed version: ubuntu-4.15.2-generic")),
	Entry("empty", "", false, errors.New("Malformed version: ")),
)

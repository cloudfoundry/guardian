package rundmc_test

import (
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/rundmc"
	"github.com/cloudfoundry-incubator/guardian/rundmc/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Rundmc", func() {
	Describe("create", func() {
		It("should ask the depot to create a container", func() {
			fakeDepot := new(fakes.FakeDepot)
			containerizer := rundmc.Containerizer{
				Depot: fakeDepot,
			}
			containerizer.Create(gardener.DesiredContainerSpec{
				Handle: "exuberant!",
			})
			Expect(fakeDepot.CreateCallCount()).To(Equal(1))
			Expect(fakeDepot.CreateArgsForCall(0)).To(Equal("exuberant!"))
		})
	})

})

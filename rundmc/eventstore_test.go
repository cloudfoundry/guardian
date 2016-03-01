package rundmc_test

import (
	"errors"
	"fmt"

	"github.com/cloudfoundry-incubator/guardian/rundmc"
	"github.com/cloudfoundry-incubator/guardian/rundmc/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Event Store", func() {
	var (
		props *fakes.FakeProperties
	)

	BeforeEach(func() {
		props = new(fakes.FakeProperties)
	})

	It("stashes events on the property manager under the 'rundmc.events' key", func() {
		events := rundmc.NewEventStore(props)
		events.OnEvent("foo", "bar")

		Expect(props.SetCallCount()).To(Equal(1))

		handle, key, value := props.SetArgsForCall(0)
		Expect(handle).To(Equal("foo"))
		Expect(key).To(Equal("rundmc.events"))
		Expect(value).To(Equal("bar"))
	})

	It("stashes further events on the same property using a CSV for the value", func() {
		props.GetReturns("bar", nil)

		events := rundmc.NewEventStore(props)
		events.OnEvent("foo", "baz")

		Expect(props.SetCallCount()).To(Equal(1))
		handle, key, value := props.SetArgsForCall(0)
		Expect(handle).To(Equal("foo"))
		Expect(key).To(Equal("rundmc.events"))
		Expect(value).To(Equal("bar,baz"))
	})

	It("retrieves events from the property manager", func() {
		props.GetStub = func(handle, key string) (string, error) {
			return fmt.Sprintf("%s,%s", handle, key), nil
		}

		events := rundmc.NewEventStore(props)
		Expect(events.Events("some-container")).To(Equal([]string{
			"some-container", "rundmc.events",
		}))
	})

	It("returns no events when the property hasn't been set or cant be retrieved", func() {
		props.GetReturns("bar", errors.New("boom"))

		events := rundmc.NewEventStore(props)
		Expect(events.Events("some-container")).To(HaveLen(0))
	})

	It("returns no events when the property is empty", func() {
		events := rundmc.NewEventStore(props)
		Expect(events.Events("some-container")).To(HaveLen(0))
	})
})

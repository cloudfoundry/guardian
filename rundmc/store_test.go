package rundmc_test

import (
	"fmt"

	"code.cloudfoundry.org/guardian/rundmc"
	fakes "code.cloudfoundry.org/guardian/rundmc/rundmcfakes"
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
		props.GetReturns("bar", true)

		events := rundmc.NewEventStore(props)
		events.OnEvent("foo", "baz")

		Expect(props.SetCallCount()).To(Equal(1))
		handle, key, value := props.SetArgsForCall(0)
		Expect(handle).To(Equal("foo"))
		Expect(key).To(Equal("rundmc.events"))
		Expect(value).To(Equal("bar,baz"))
	})

	It("retrieves events from the property manager", func() {
		props.GetStub = func(handle, key string) (string, bool) {
			return fmt.Sprintf("%s,%s", handle, key), true
		}

		events := rundmc.NewEventStore(props)
		Expect(events.Events("some-container")).To(Equal([]string{
			"some-container", "rundmc.events",
		}))
	})

	It("returns no events when the property hasn't been set or cant be retrieved", func() {
		props.GetReturns("bar", false)

		events := rundmc.NewEventStore(props)
		Expect(events.Events("some-container")).To(HaveLen(0))
	})

	It("returns no events when the property is empty", func() {
		events := rundmc.NewEventStore(props)
		Expect(events.Events("some-container")).To(HaveLen(0))
	})
})

var _ = Describe("States Store", func() {
	var (
		props *fakes.FakeProperties
	)

	BeforeEach(func() {
		props = new(fakes.FakeProperties)
	})

	It("stashes the state on the property manager under the 'rundmc.state' key", func() {
		states := rundmc.NewStateStore(props)
		states.StoreStopped("foo")

		Expect(props.SetCallCount()).To(Equal(1))

		handle, key, value := props.SetArgsForCall(0)
		Expect(handle).To(Equal("foo"))
		Expect(key).To(Equal("rundmc.state"))
		Expect(value).To(Equal("stopped"))
	})

	Describe("IsStopped", func() {
		var (
			state map[string]string
		)

		BeforeEach(func() {
			state = make(map[string]string)

			props.GetStub = func(handle, key string) (string, bool) {
				Expect(handle).To(Equal("some-handle"))
				v, ok := state[key]
				return v, ok
			}
		})

		Context("when the rundmc.state has the value 'stopped'", func() {
			BeforeEach(func() {
				state["rundmc.state"] = "stopped"
			})

			It("returns true", func() {
				states := rundmc.NewStateStore(props)
				Expect(states.IsStopped("some-handle")).To(BeTrue())
			})
		})

		Context("when the rundmc.state has a value other than 'stopped'", func() {
			BeforeEach(func() {
				state["rundmc.state"] = "potato"
			})

			It("returns false", func() {
				states := rundmc.NewStateStore(props)
				Expect(states.IsStopped("some-handle")).To(BeFalse())
			})
		})

		Context("when the rundmc.state has no value", func() {
			It("returns false", func() {
				states := rundmc.NewStateStore(props)
				Expect(states.IsStopped("some-handle")).To(BeFalse())
			})
		})
	})
})

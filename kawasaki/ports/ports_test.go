package ports_test

import (
	"code.cloudfoundry.org/guardian/kawasaki/ports"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Port pool", func() {
	var initialState ports.State

	BeforeEach(func() {
		initialState = ports.State{Offset: 0}
	})

	Describe("initialization", func() {
		Context("when port range exeeding Linux limit given", func() {
			It("will return an error", func() {
				_, err := ports.NewPool(61001, 5000, initialState)
				Expect(err).To(MatchError(ContainSubstring("invalid port range")))
			})
		})
	})

	Describe("acquiring", func() {
		It("returns the next available port from the pool", func() {
			pool, err := ports.NewPool(10000, 5, initialState)
			Expect(err).ToNot(HaveOccurred())

			port1, err := pool.Acquire()
			Expect(err).ToNot(HaveOccurred())

			port2, err := pool.Acquire()
			Expect(err).ToNot(HaveOccurred())

			Expect(port1).To(Equal(uint32(10000)))
			Expect(port2).To(Equal(uint32(10001)))
		})

		Context("when the pool is exhausted", func() {
			It("returns an error", func() {
				pool, err := ports.NewPool(10000, 5, initialState)
				Expect(err).ToNot(HaveOccurred())

				for i := 0; i < 5; i++ {
					_, err := pool.Acquire()
					Expect(err).ToNot(HaveOccurred())
				}

				_, err = pool.Acquire()
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the offset is positive", func() {
			It("returns the next available port while honoring the offset", func() {
				initialState.Offset = 2
				pool, err := ports.NewPool(10000, 5, initialState)
				Expect(err).ToNot(HaveOccurred())

				port1, err := pool.Acquire()
				Expect(err).ToNot(HaveOccurred())

				port2, err := pool.Acquire()
				Expect(err).ToNot(HaveOccurred())

				Expect(port1).To(Equal(uint32(10002)))
				Expect(port2).To(Equal(uint32(10003)))
			})

			Context("when offset is greater than the size", func() {
				It("returns the first port of the range", func() {
					pool, err := ports.NewPool(10000, 5, initialState)
					Expect(err).ToNot(HaveOccurred())

					port, err := pool.Acquire()
					Expect(err).ToNot(HaveOccurred())
					Expect(port).To(Equal(uint32(10000)))
				})
			})

			It("acquired already used ports", func() {
				startPort := uint32(10000)
				portOffset := uint32(4)
				initialState.Offset = portOffset

				pool, err := ports.NewPool(startPort, 5, initialState)
				Expect(err).ToNot(HaveOccurred())

				port, err := pool.Acquire()
				Expect(port).To(Equal(uint32(10004)))
				Expect(err).ToNot(HaveOccurred())

				for i := uint32(0); i < portOffset; i++ {
					port, err := pool.Acquire()
					Expect(err).ToNot(HaveOccurred())
					Expect(port).To(Equal(startPort + i))
				}
			})
		})
	})

	Describe("removing", func() {
		It("acquires a specific port from the pool", func() {
			pool, err := ports.NewPool(10000, 2, initialState)
			Expect(err).ToNot(HaveOccurred())

			err = pool.Remove(10000)
			Expect(err).ToNot(HaveOccurred())

			port, err := pool.Acquire()
			Expect(err).ToNot(HaveOccurred())
			Expect(port).To(Equal(uint32(10001)))

			_, err = pool.Acquire()
			Expect(err).To(HaveOccurred())
		})

		Context("when the resource is already acquired", func() {
			It("returns a PortTakenError", func() {
				pool, err := ports.NewPool(10000, 2, initialState)
				Expect(err).ToNot(HaveOccurred())

				port, err := pool.Acquire()
				Expect(err).ToNot(HaveOccurred())

				err = pool.Remove(port)
				Expect(err).To(Equal(ports.PortTakenError{Port: port}))
			})
		})
	})

	Describe("releasing", func() {
		It("places a port back at the end of the pool", func() {
			pool, err := ports.NewPool(10000, 2, initialState)
			Expect(err).ToNot(HaveOccurred())

			port1, err := pool.Acquire()
			Expect(err).ToNot(HaveOccurred())
			Expect(port1).To(Equal(uint32(10000)))

			pool.Release(port1)

			port2, err := pool.Acquire()
			Expect(err).ToNot(HaveOccurred())
			Expect(port2).To(Equal(uint32(10001)))

			nextPort, err := pool.Acquire()
			Expect(err).ToNot(HaveOccurred())
			Expect(nextPort).To(Equal(uint32(10000)))
		})

		Context("when the released port is out of the range", func() {
			It("does not add it to the pool", func() {
				pool, err := ports.NewPool(10000, 0, initialState)
				Expect(err).ToNot(HaveOccurred())

				pool.Release(20000)

				_, err = pool.Acquire()
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the released port is already released", func() {
			It("does not duplicate it", func() {
				pool, err := ports.NewPool(10000, 2, initialState)
				Expect(err).ToNot(HaveOccurred())

				port1, err := pool.Acquire()
				Expect(err).ToNot(HaveOccurred())
				Expect(port1).To(Equal(uint32(10000)))

				pool.Release(port1)
				pool.Release(port1)

				port2, err := pool.Acquire()
				Expect(err).ToNot(HaveOccurred())
				Expect(port2).ToNot(Equal(port1))

				port3, err := pool.Acquire()
				Expect(err).ToNot(HaveOccurred())
				Expect(port3).To(Equal(port1))

				_, err = pool.Acquire()
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("RefreshState", func() {
		It("returns the state with the appropriate offset", func() {
			pool, err := ports.NewPool(10000, 5, initialState)
			Expect(err).ToNot(HaveOccurred())

			_, err = pool.Acquire()
			Expect(err).NotTo(HaveOccurred())

			newState := pool.RefreshState()
			Expect(newState.Offset).To(BeNumerically("==", 1))
		})

		Context("when port pool is exhausted", func() {
			It("returns the state reset to offset 0", func() {
				pool, err := ports.NewPool(10000, 1, initialState)
				Expect(err).ToNot(HaveOccurred())

				_, err = pool.Acquire()
				Expect(err).NotTo(HaveOccurred())

				newState := pool.RefreshState()
				Expect(newState.Offset).To(BeNumerically("==", 0))
			})
		})
	})
})

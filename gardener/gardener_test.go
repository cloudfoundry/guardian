package gardener_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/gardener/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Gardener", func() {
	var (
		containerizer *fakes.FakeContainerizer
		gdnr          *gardener.Gardener
	)

	BeforeEach(func() {
		containerizer = new(fakes.FakeContainerizer)
		gdnr = &gardener.Gardener{
			Containerizer: containerizer,
		}
	})

	Describe("creating a container", func() {
		It("asks the containerizer to create a container", func() {
			_, err := gdnr.Create(garden.ContainerSpec{
				Handle: "bob",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(containerizer.CreateCallCount()).To(Equal(1))
			Expect(containerizer.CreateArgsForCall(0)).To(Equal(gardener.DesiredContainerSpec{
				Handle: "bob",
			}))
		})
	})

	Context("when having a container", func() {
		var container garden.Container

		BeforeEach(func() {
			var err error
			container, err = gdnr.Lookup("banana")
			Expect(err).NotTo(HaveOccurred())
		})

		Describe("running a process in a container", func() {
			It("asks the containerizer to run the process", func() {
				origSpec := garden.ProcessSpec{Path: "ripe"}
				origIO := garden.ProcessIO{
					Stdout: gbytes.NewBuffer(),
				}
				_, err := container.Run(origSpec, origIO)
				Expect(err).ToNot(HaveOccurred())

				Expect(containerizer.RunCallCount()).To(Equal(1))
				id, spec, io := containerizer.RunArgsForCall(0)
				Expect(id).To(Equal("banana"))
				Expect(spec).To(Equal(origSpec))
				Expect(io).To(Equal(origIO))
			})

			Context("when the containerizer fails to run a process", func() {
				BeforeEach(func() {
					containerizer.RunReturns(nil, errors.New("lost my banana"))
				})

				It("returns the error", func() {
					_, err := container.Run(garden.ProcessSpec{}, garden.ProcessIO{})
					Expect(err).To(MatchError("lost my banana"))
				})
			})
		})
	})
})

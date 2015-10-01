package rundmc_test

import (
	"errors"
	"io"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/rundmc"
	"github.com/cloudfoundry-incubator/guardian/rundmc/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Rundmc", func() {
	var (
		fakeDepot           *fakes.FakeDepot
		fakeBundler         *fakes.FakeBundler
		fakeContainerRunner *fakes.FakeBundleRunner
		fakeStartCheck      *fakes.FakeChecker

		containerizer *rundmc.Containerizer
	)

	BeforeEach(func() {
		fakeDepot = new(fakes.FakeDepot)
		fakeContainerRunner = new(fakes.FakeBundleRunner)
		fakeStartCheck = new(fakes.FakeChecker)
		fakeBundler = new(fakes.FakeBundler)

		containerizer = rundmc.New(fakeDepot, fakeBundler, fakeContainerRunner, fakeStartCheck)

		fakeDepot.LookupStub = func(handle string) (string, error) {
			return "/path/to/" + handle, nil
		}
	})

	Describe("create", func() {
		It("should ask the depot to create a container", func() {
			var returnedBundle *goci.Bndl
			fakeBundler.BundleStub = func(spec gardener.DesiredContainerSpec) *goci.Bndl {
				returnedBundle = goci.Bundle().WithRootFS(spec.NetworkPath)
				return returnedBundle
			}

			containerizer.Create(gardener.DesiredContainerSpec{
				Handle: "exuberant!",
			})

			Expect(fakeDepot.CreateCallCount()).To(Equal(1))

			handle, bundle := fakeDepot.CreateArgsForCall(0)
			Expect(handle).To(Equal("exuberant!"))
			Expect(bundle).To(Equal(returnedBundle))
		})

		Context("when creating the depot directory fails", func() {
			It("returns an error", func() {
				fakeDepot.CreateReturns(errors.New("blam"))
				Expect(containerizer.Create(gardener.DesiredContainerSpec{
					Handle: "exuberant!",
				})).NotTo(Succeed())
			})
		})

		Context("when looking up the container fails", func() {
			It("returns an error", func() {
				fakeDepot.LookupReturns("", errors.New("blam"))
				Expect(containerizer.Create(gardener.DesiredContainerSpec{
					Handle: "exuberant!",
				})).NotTo(Succeed())
			})

			It("does not attempt to start the container", func() {
				fakeDepot.LookupReturns("", errors.New("blam"))
				containerizer.Create(gardener.DesiredContainerSpec{
					Handle: "exuberant!",
				})

				Expect(fakeContainerRunner.StartCallCount()).To(Equal(0))
			})
		})

		It("should start a container in the created directory", func() {
			containerizer.Create(gardener.DesiredContainerSpec{
				Handle: "exuberant!",
			})

			Expect(fakeContainerRunner.StartCallCount()).To(Equal(1))

			path, id, _ := fakeContainerRunner.StartArgsForCall(0)
			Expect(path).To(Equal("/path/to/exuberant!"))
			Expect(id).To(Equal("exuberant!"))
		})

		Describe("waiting for the container to start", func() {
			Context("when the container starts succesfully", func() {
				It("returns success", func() {
					fakeStartCheck.CheckReturns(nil)
					Expect(containerizer.Create(gardener.DesiredContainerSpec{})).To(Succeed())
				})
			})

			Context("when the container fails to start", func() {
				It("returns the underlying error", func() {
					fakeStartCheck.CheckStub = func(stdout io.Reader) error {
						return errors.New("I died")
					}

					Expect(containerizer.Create(gardener.DesiredContainerSpec{})).To(MatchError("I died"))
				})
			})
		})
	})

	Describe("run", func() {
		It("should ask the execer to exec a process in the container", func() {
			containerizer.Run("some-handle", garden.ProcessSpec{Path: "hello"}, garden.ProcessIO{})
			Expect(fakeContainerRunner.ExecCallCount()).To(Equal(1))

			id, spec, _ := fakeContainerRunner.ExecArgsForCall(0)
			Expect(id).To(Equal("some-handle"))
			Expect(spec.Path).To(Equal("hello"))
		})

		Context("when looking up the container fails", func() {
			It("returns an error", func() {
				fakeDepot.LookupReturns("", errors.New("blam"))
				_, err := containerizer.Run("some-handle", garden.ProcessSpec{}, garden.ProcessIO{})
				Expect(err).To(HaveOccurred())
			})

			It("does not attempt to exec the process", func() {
				fakeDepot.LookupReturns("", errors.New("blam"))
				containerizer.Run("some-handle", garden.ProcessSpec{}, garden.ProcessIO{})
				Expect(fakeContainerRunner.StartCallCount()).To(Equal(0))
			})
		})
	})

	Describe("destroy", func() {
		It("should run kill", func() {
			Expect(containerizer.Destroy("some-handle")).To(Succeed())
			Expect(fakeContainerRunner.KillCallCount()).To(Equal(1))
			Expect(fakeContainerRunner.KillArgsForCall(0)).To(Equal("/path/to/some-handle"))
		})

		Context("when kill succeeds", func() {
			It("destroys the depot directory", func() {
				Expect(containerizer.Destroy("some-handle")).To(Succeed())
				Expect(fakeDepot.DestroyCallCount()).To(Equal(1))
				Expect(fakeDepot.DestroyArgsForCall(0)).To(Equal("some-handle"))
			})
		})

		Context("when kill fails", func() {
			It("does not destroy the depot directory", func() {
				fakeContainerRunner.KillReturns(errors.New("killing is wrong"))
				containerizer.Destroy("some-handle")
				Expect(fakeDepot.DestroyCallCount()).To(Equal(0))
			})
		})
	})
})

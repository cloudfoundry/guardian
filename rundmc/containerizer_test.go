package rundmc_test

import (
	"errors"
	"io"
	"os"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/rundmc"
	"github.com/cloudfoundry-incubator/guardian/rundmc/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Rundmc", func() {
	var (
		fakeDepot           *fakes.FakeDepot
		fakeBundler         *fakes.FakeBundleGenerator
		fakeContainerRunner *fakes.FakeBundleRunner
		fakeStartChecker    *fakes.FakeChecker
		fakeNstarRunner     *fakes.FakeNstarRunner
		fakeStater          *fakes.FakeContainerStater
		logger              lager.Logger

		containerizer *rundmc.Containerizer
	)

	BeforeEach(func() {
		fakeDepot = new(fakes.FakeDepot)
		fakeContainerRunner = new(fakes.FakeBundleRunner)
		fakeStartChecker = new(fakes.FakeChecker)
		fakeBundler = new(fakes.FakeBundleGenerator)
		fakeNstarRunner = new(fakes.FakeNstarRunner)
		fakeStater = new(fakes.FakeContainerStater)
		logger = lagertest.NewTestLogger("test")

		containerizer = rundmc.New(fakeDepot, fakeBundler, fakeContainerRunner, fakeStartChecker, fakeStater, fakeNstarRunner)

		fakeDepot.LookupStub = func(_ lager.Logger, handle string) (string, error) {
			return "/path/to/" + handle, nil
		}
	})

	Describe("Create", func() {
		It("should ask the depot to create a container", func() {
			var returnedBundle *goci.Bndl
			fakeBundler.GenerateStub = func(spec gardener.DesiredContainerSpec) *goci.Bndl {
				return returnedBundle
			}

			containerizer.Create(logger, gardener.DesiredContainerSpec{
				Handle: "exuberant!",
			})

			Expect(fakeDepot.CreateCallCount()).To(Equal(1))

			_, handle, bundle := fakeDepot.CreateArgsForCall(0)
			Expect(handle).To(Equal("exuberant!"))
			Expect(bundle).To(Equal(returnedBundle))
		})

		Context("when creating the depot directory fails", func() {
			It("returns an error", func() {
				fakeDepot.CreateReturns(errors.New("blam"))
				Expect(containerizer.Create(logger, gardener.DesiredContainerSpec{
					Handle: "exuberant!",
				})).NotTo(Succeed())
			})
		})

		It("should start a container in the created directory", func() {
			Expect(containerizer.Create(logger, gardener.DesiredContainerSpec{
				Handle: "exuberant!",
			})).To(Succeed())

			Expect(fakeContainerRunner.StartCallCount()).To(Equal(1))

			_, path, id, _ := fakeContainerRunner.StartArgsForCall(0)
			Expect(path).To(Equal("/path/to/exuberant!"))
			Expect(id).To(Equal("exuberant!"))
		})

		It("should prepare the root file system", func() {
			Expect(containerizer.Create(logger, gardener.DesiredContainerSpec{
				Handle: "exuberant!",
			})).To(Succeed())

		})

		Context("when the container fails to start", func() {
			BeforeEach(func() {
				fakeContainerRunner.StartReturns(nil, errors.New("banana"))
			})

			It("should return an error", func() {
				Expect(containerizer.Create(logger, gardener.DesiredContainerSpec{})).NotTo(Succeed())
			})

			It("should not check if the container is started", func() {
				Expect(containerizer.Create(logger, gardener.DesiredContainerSpec{})).NotTo(Succeed())
				Expect(fakeStartChecker.CheckCallCount()).To(Equal(0))
			})
		})

		It("should check if the container is started", func() {
			Expect(containerizer.Create(logger, gardener.DesiredContainerSpec{})).To(Succeed())
			Expect(fakeStartChecker.CheckCallCount()).To(Equal(1))
		})

		Context("when the start check fails", func() {
			It("returns the underlying error", func() {
				fakeStartChecker.CheckStub = func(_ lager.Logger, stdout io.Reader) error {
					return errors.New("I died")
				}

				Expect(containerizer.Create(logger, gardener.DesiredContainerSpec{Handle: "the-handle"})).To(MatchError("I died"))
			})
		})
	})

	Describe("Run", func() {
		It("should ask the execer to exec a process in the container", func() {
			containerizer.Run(logger, "some-handle", garden.ProcessSpec{Path: "hello"}, garden.ProcessIO{})
			Expect(fakeContainerRunner.ExecCallCount()).To(Equal(1))

			_, path, id, spec, _ := fakeContainerRunner.ExecArgsForCall(0)
			Expect(path).To(Equal("/path/to/some-handle"))
			Expect(id).To(Equal("some-handle"))
			Expect(spec.Path).To(Equal("hello"))
		})

		Context("when looking up the container fails", func() {
			It("returns an error", func() {
				fakeDepot.LookupReturns("", errors.New("blam"))
				_, err := containerizer.Run(logger, "some-handle", garden.ProcessSpec{}, garden.ProcessIO{})
				Expect(err).To(HaveOccurred())
			})

			It("does not attempt to exec the process", func() {
				fakeDepot.LookupReturns("", errors.New("blam"))
				containerizer.Run(logger, "some-handle", garden.ProcessSpec{}, garden.ProcessIO{})
				Expect(fakeContainerRunner.StartCallCount()).To(Equal(0))
			})
		})
	})

	Describe("StreamIn", func() {
		It("should execute the NSTar command with the container PID", func() {
			fakeStater.StateReturns(rundmc.State{
				Pid: 12,
			}, nil)

			someStream := gbytes.NewBuffer()
			Expect(containerizer.StreamIn(logger, "some-handle", garden.StreamInSpec{
				Path:      "some-path",
				User:      "some-user",
				TarStream: someStream,
			})).To(Succeed())

			_, pid, path, user, stream := fakeNstarRunner.StreamInArgsForCall(0)
			Expect(pid).To(Equal(12))
			Expect(path).To(Equal("some-path"))
			Expect(user).To(Equal("some-user"))
			Expect(stream).To(Equal(someStream))
		})

		It("returns an error if the PID cannot be found", func() {
			fakeStater.StateReturns(rundmc.State{}, errors.New("pid not found"))
			Expect(containerizer.StreamIn(logger, "some-handle", garden.StreamInSpec{})).To(MatchError("stream-in: pid not found for container"))
		})

		It("returns the error if nstar fails", func() {
			fakeNstarRunner.StreamInReturns(errors.New("failed"))
			Expect(containerizer.StreamIn(logger, "some-handle", garden.StreamInSpec{})).To(MatchError("stream-in: nstar: failed"))
		})
	})

	Describe("StreamOut", func() {
		It("should execute the NSTar command with the container PID", func() {
			fakeStater.StateReturns(rundmc.State{
				Pid: 12,
			}, nil)

			fakeNstarRunner.StreamOutReturns(os.Stdin, nil)

			tarStream, err := containerizer.StreamOut(logger, "some-handle", garden.StreamOutSpec{
				Path: "some-path",
				User: "some-user",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(tarStream).To(Equal(os.Stdin))

			_, pid, path, user := fakeNstarRunner.StreamOutArgsForCall(0)
			Expect(pid).To(Equal(12))
			Expect(path).To(Equal("some-path"))
			Expect(user).To(Equal("some-user"))
		})

		It("returns an error if the PID cannot be found", func() {
			fakeStater.StateReturns(rundmc.State{}, errors.New("pid not found"))
			tarStream, err := containerizer.StreamOut(logger, "some-handle", garden.StreamOutSpec{})

			Expect(tarStream).To(BeNil())
			Expect(err).To(MatchError("stream-out: pid not found for container"))
		})

		It("returns the error if nstar fails", func() {
			fakeNstarRunner.StreamOutReturns(nil, errors.New("failed"))
			tarStream, err := containerizer.StreamOut(logger, "some-handle", garden.StreamOutSpec{})

			Expect(tarStream).To(BeNil())
			Expect(err).To(MatchError("stream-out: nstar: failed"))
		})
	})

	Describe("destroy", func() {
		It("should run kill", func() {
			Expect(containerizer.Destroy(logger, "some-handle")).To(Succeed())
			Expect(fakeContainerRunner.KillCallCount()).To(Equal(1))
			Expect(arg2(fakeContainerRunner.KillArgsForCall(0))).To(Equal("some-handle"))
		})

		Context("when kill succeeds", func() {
			It("destroys the depot directory", func() {
				Expect(containerizer.Destroy(logger, "some-handle")).To(Succeed())
				Expect(fakeDepot.DestroyCallCount()).To(Equal(1))
				Expect(arg2(fakeDepot.DestroyArgsForCall(0))).To(Equal("some-handle"))
			})
		})

		Context("when kill fails", func() {
			It("does not destroy the depot directory", func() {
				fakeContainerRunner.KillReturns(errors.New("killing is wrong"))
				containerizer.Destroy(logger, "some-handle")
				Expect(fakeDepot.DestroyCallCount()).To(Equal(0))
			})
		})
	})

	Describe("Info", func() {
		It("should return the ActualContainerSpec with the correct bundlePath", func() {
			actualSpec, err := containerizer.Info(logger, "some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSpec.BundlePath).To(Equal("/path/to/some-handle"))
		})

		Context("when the lookup fails", func() {
			It("should return the error", func() {
				fakeDepot.LookupReturns("", errors.New("spiderman-error"))
				_, err := containerizer.Info(logger, "some-handle")
				Expect(err).To(MatchError("spiderman-error"))
			})
		})
	})

	Describe("handles", func() {
		Context("when handles exist", func() {
			BeforeEach(func() {
				fakeDepot.HandlesReturns([]string{"banana", "banana2"}, nil)
			})

			It("should return the handles", func() {
				Expect(containerizer.Handles()).To(ConsistOf("banana", "banana2"))
			})
		})

		Context("when the depot returns an error", func() {
			testErr := errors.New("spiderman error")

			BeforeEach(func() {
				fakeDepot.HandlesReturns([]string{}, testErr)
			})

			It("should return the error", func() {
				_, err := containerizer.Handles()
				Expect(err).To(MatchError(testErr))
			})
		})
	})
})

func arg2(_ lager.Logger, i interface{}) interface{} {
	return i
}

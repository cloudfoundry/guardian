package gardener_test

import (
	"errors"
	"net/url"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	fakes "code.cloudfoundry.org/guardian/gardener/gardenerfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("VolumeProvider", func() {
	var (
		volumeCreator  *fakes.FakeVolumeCreator
		volumeProvider *gardener.VolumeProvider
		cmdRunner      *fake_command_runner.FakeCommandRunner
		logger         lager.Logger
	)

	BeforeEach(func() {
		volumeCreator = new(fakes.FakeVolumeCreator)
		cmdRunner = new(fake_command_runner.FakeCommandRunner)
		volumeProvider = gardener.NewVolumeProvider(volumeCreator, nil, cmdRunner, 5, 5)
		logger = lagertest.NewTestLogger("volume-provider-test")
	})

	Describe("Create", func() {
		var (
			containerSpec garden.ContainerSpec
			runtimeSpec   specs.Spec
		)

		BeforeEach(func() {
			containerSpec = garden.ContainerSpec{
				Image: garden.ImageRef{URI: "raw:///path/to/some/rootfs"},
			}
		})

		Describe("success", func() {
			JustBeforeEach(func() {
				var err error
				runtimeSpec, err = volumeProvider.Create(logger, containerSpec)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when passed a raw rootfs", func() {
				It("returns the raw root path in the runtime spec", func() {
					Expect(runtimeSpec.Root.Path).To(Equal("/path/to/some/rootfs"))
				})

				It("doesn't call the VolumeCreator", func() {
					Expect(volumeCreator.CreateCallCount()).To(Equal(0))
				})
			})

			Context("when the deprecated RootfsPath is specified instead of the Image", func() {
				BeforeEach(func() {
					containerSpec = garden.ContainerSpec{RootFSPath: "raw:///some/rootfs"}
				})

				It("returns the rootfs path in the runtime spec", func() {
					Expect(runtimeSpec.Root.Path).To(Equal("/some/rootfs"))
				})
			})

			Context("when passed a non-raw rootfs", func() {
				BeforeEach(func() {
					containerSpec = garden.ContainerSpec{
						Handle: "some-handle",
						Image: garden.ImageRef{
							URI:      "docker:///alpine#3.7",
							Username: "cakeuser",
							Password: "cakepassword",
						},
						Limits: garden.Limits{
							Disk: garden.DiskLimits{
								Scope:    garden.DiskLimitScopeTotal,
								ByteHard: 10000,
							},
						},
						Privileged: true,
					}

					volumeCreator.CreateReturns(specs.Spec{Version: "best-spec", Root: &specs.Root{Path: "/hello"}}, nil)
				})

				It("calls the VolumeCreator with the correct parameters", func() {
					Expect(volumeCreator.CreateCallCount()).To(Equal(1))
					_, handle, rootfsSpec := volumeCreator.CreateArgsForCall(0)
					Expect(handle).To(Equal("some-handle"))

					parsedRootFS, err := url.Parse("docker:///alpine#3.7")
					Expect(err).NotTo(HaveOccurred())
					Expect(rootfsSpec).To(Equal(gardener.RootfsSpec{
						RootFS:     parsedRootFS,
						Username:   "cakeuser",
						Password:   "cakepassword",
						QuotaSize:  10000,
						QuotaScope: garden.DiskLimitScopeTotal,
						Namespaced: false,
					}))
				})

				It("returns the runtime spec from the VolumeCreator", func() {
					Expect(runtimeSpec).To(Equal(specs.Spec{Version: "best-spec", Root: &specs.Root{Path: "/hello"}}))
				})
			})
		})

		Describe("failure", func() {
			var createErr error

			JustBeforeEach(func() {
				_, createErr = volumeProvider.Create(logger, containerSpec)
			})

			Context("when passing both an Image and a rootfsPath", func() {
				BeforeEach(func() {
					containerSpec = garden.ContainerSpec{
						Image:      garden.ImageRef{URI: "raw:///path/to/some/rootfs"},
						RootFSPath: "/path/to/some/rootfs",
					}
				})

				It("returns an error", func() {
					Expect(createErr).To(MatchError("Cannot provide both Image.URI and RootFSPath"))
				})
			})

			Context("when the VolumeCreator errors", func() {
				BeforeEach(func() {
					containerSpec = garden.ContainerSpec{
						Image: garden.ImageRef{URI: "/path/to/some/rootfs"},
					}
					volumeCreator.CreateReturns(specs.Spec{}, errors.New("volume-create-error"))
				})

				It("returns the error", func() {
					Expect(createErr).To(MatchError("volume-create-error"))
				})
			})

			Context("when the Image URI is invalid", func() {
				BeforeEach(func() {
					containerSpec = garden.ContainerSpec{
						Image: garden.ImageRef{URI: "://-!?"},
					}
				})

				It("returns an error", func() {
					Expect(createErr).To(HaveOccurred())
				})
			})
		})
	})
})

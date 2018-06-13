package peas_test

import (
	"errors"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/peas"
	"code.cloudfoundry.org/guardian/rundmc/peas/peasfakes"
	"code.cloudfoundry.org/guardian/rundmc/rundmcfakes"
	"code.cloudfoundry.org/guardian/rundmc/users"
	"code.cloudfoundry.org/guardian/rundmc/users/usersfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("PeaUsernameResolver", func() {
	var (
		bundle goci.Bndl

		resolvedUid int
		resolvedGid int
		resolveErr  error

		pidGetter          *peasfakes.FakeProcessPidGetter
		peaCreator         *rundmcfakes.FakePeaCreator
		loader             *rundmcfakes.FakeBundleLoader
		userLookupper      *usersfakes.FakeUserLookupper
		userResolveProcess *gardenfakes.FakeProcess

		resolver peas.PeaUsernameResolver
	)

	BeforeEach(func() {
		pidGetter = new(peasfakes.FakeProcessPidGetter)
		peaCreator = new(rundmcfakes.FakePeaCreator)
		loader = new(rundmcfakes.FakeBundleLoader)
		userLookupper = new(usersfakes.FakeUserLookupper)
		userResolveProcess = new(gardenfakes.FakeProcess)

		bundle = goci.Bndl{
			Spec: specs.Spec{
				Mounts: []specs.Mount{
					specs.Mount{Source: "/path/to/some/mount", Destination: "/mount"},
					specs.Mount{Source: "/path/to/host/init", Destination: "/tmp/garden-init", Type: "bind"},
				},
				Process: &specs.Process{
					Args: []string{"/path/to/process"},
				},
			},
		}
		loader.LoadReturns(bundle, nil)

		userResolveProcess.IDReturns("peaid")
		peaCreator.CreatePeaReturns(userResolveProcess, nil)

		userLookupper.LookupReturns(&users.ExecUser{Uid: 1, Gid: 2}, nil)

		pidGetter.PidReturns(42, nil)

		resolver = peas.PeaUsernameResolver{
			PidGetter:     pidGetter,
			PeaCreator:    peaCreator,
			Loader:        loader,
			UserLookupper: userLookupper,
		}
	})

	JustBeforeEach(func() {
		resolvedUid, resolvedGid, resolveErr = resolver.ResolveUser(lagertest.NewTestLogger(""), "/path/to/bundle", "handle", garden.ImageRef{URI: "image-uri"}, "foobar")
	})

	It("resolves username", func() {
		Expect(resolveErr).NotTo(HaveOccurred())
		Expect(resolvedUid).To(Equal(1))
		Expect(resolvedGid).To(Equal(2))
	})

	It("resolves username against correct rootfs", func() {
		Expect(userLookupper.LookupCallCount()).To(Equal(1))
		rootfs, username := userLookupper.LookupArgsForCall(0)
		Expect(rootfs).To(Equal(toFilePath("/proc/42/root")))
		Expect(username).To(Equal("foobar"))

		Expect(pidGetter.PidCallCount()).To(Equal(1))
		pidFilePath := pidGetter.PidArgsForCall(0)
		Expect(pidFilePath).To(Equal(toFilePath("/path/to/bundle/processes/peaid/pidfile")))
	})

	It("creates the resolve user helper pea with the correct params", func() {
		Expect(peaCreator.CreatePeaCallCount()).To(Equal(1))
		_, processSpec, _, handle, bundlePath := peaCreator.CreatePeaArgsForCall(0)
		Expect(processSpec.Path).To(Equal("/path/to/process"))
		Expect(processSpec.User).To(Equal("0:0"))
		Expect(processSpec.BindMounts).To(ConsistOf(
			garden.BindMount{
				SrcPath: "/path/to/host/init",
				DstPath: "/tmp/garden-init",
			},
		))
		Expect(processSpec.Image).To(Equal(garden.ImageRef{URI: "image-uri"}))
		Expect(handle).To(Equal("handle"))
		Expect(bundlePath).To(Equal("/path/to/bundle"))
	})

	It("kills the resolve user helper pea", func() {
		Expect(userResolveProcess.SignalCallCount()).To(Equal(1))
		signal := userResolveProcess.SignalArgsForCall(0)
		Expect(signal).To(Equal(garden.SignalKill))
	})

	Context("when bundle cannot be loaded", func() {
		BeforeEach(func() {
			loader.LoadReturns(goci.Bndl{}, errors.New("bundle-load-failure"))
		})

		It("returns an error", func() {
			Expect(resolveErr).To(MatchError("bundle-load-failure"))
		})
	})

	Context("when garden-init bind mount cannot be found", func() {
		BeforeEach(func() {
			bundle.Spec.Mounts = []specs.Mount{}
			loader.LoadReturns(bundle, nil)
		})

		It("returns an error", func() {
			Expect(resolveErr).To(MatchError("Could not find bind mount to /tmp/garden-init"))
		})
	})

	Context("when resolve user helper pea cannot be created", func() {
		BeforeEach(func() {
			peaCreator.CreatePeaReturns(nil, errors.New("create-pea-failure"))
		})

		It("returns an error", func() {
			Expect(resolveErr).To(MatchError("create-pea-failure"))
		})
	})

	Context("when resolve user helper pea init pid cannot be resolved", func() {
		BeforeEach(func() {
			pidGetter.PidReturns(-1, errors.New("get-pid-failure"))
		})

		It("returns an error", func() {
			Expect(resolveErr).To(MatchError("get-pid-failure"))
		})
	})

	Context("when user cannot be looked up", func() {
		BeforeEach(func() {
			userLookupper.LookupReturns(nil, errors.New("user-lookup-failure"))
		})

		It("returns an error", func() {
			Expect(resolveErr).To(MatchError("user-lookup-failure"))
		})
	})
})

func toFilePath(unixPath string) string {
	subPaths := strings.Split(unixPath, "/")
	return filepath.Join("/", filepath.Join(subPaths...))
}

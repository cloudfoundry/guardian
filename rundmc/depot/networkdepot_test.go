package depot_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	fakes "code.cloudfoundry.org/guardian/rundmc/depot/depotfakes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NetworkDepot", func() {
	var (
		dir                    string
		bindMountSourceCreator *fakes.FakeBindMountSourceCreator
		logger                 lager.Logger
		networkDepot           depot.NetworkDepot
	)

	BeforeEach(func() {
		var err error
		dir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		logger = lagertest.NewTestLogger("test")
		bindMountSourceCreator = new(fakes.FakeBindMountSourceCreator)
		networkDepot = depot.NewNetworkDepot(dir, bindMountSourceCreator)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(dir)).To(Succeed())
	})

	Describe("SetupBindMounts", func() {
		var (
			bindMounts []garden.BindMount
			setupErr   error
		)

		BeforeEach(func() {
			bindMountSourceCreator.CreateReturns([]garden.BindMount{{SrcPath: "src"}}, nil)
		})

		JustBeforeEach(func() {
			bindMounts, setupErr = networkDepot.SetupBindMounts(logger, "my-container", true, "/path/to/rootfs")
		})

		It("succeeds", func() {
			Expect(setupErr).NotTo(HaveOccurred())
		})

		It("creates container directory", func() {
			containerDir := filepath.Join(dir, "my-container")
			Expect(containerDir).To(BeADirectory())
		})

		It("creates the network bind mounts", func() {
			Expect(bindMountSourceCreator.CreateCallCount()).To(Equal(1))
			actualContainerDir, actualPrivileged := bindMountSourceCreator.CreateArgsForCall(0)
			Expect(actualContainerDir).To(Equal(filepath.Join(dir, "my-container")))
			Expect(actualPrivileged).To(BeFalse())
			Expect(bindMounts).To(ConsistOf(garden.BindMount{SrcPath: "src"}))
		})

		Context("when creating the network bind mounts fails", func() {
			BeforeEach(func() {
				bindMountSourceCreator.CreateStub = func(containerDir string, _ bool) ([]garden.BindMount, error) {
					Expect(ioutil.WriteFile(filepath.Join(containerDir, "hosts"), nil, 0755)).To(Succeed())
					Expect(ioutil.WriteFile(filepath.Join(containerDir, "resolv.conf"), nil, 0755)).To(Succeed())
					return nil, errors.New("failed")
				}
			})

			It("returns the error", func() {
				Expect(setupErr).To(MatchError("failed"))
			})

			It("deletes the bind mounts from the depot", func() {
				containerDir := filepath.Join(dir, "my-container")
				Expect(containerDir).To(BeADirectory())
				Expect(filepath.Join(containerDir, "hosts")).NotTo(BeARegularFile())
				Expect(filepath.Join(containerDir, "resolv.conf")).NotTo(BeARegularFile())
			})
		})
	})

	Describe("Destroy", func() {
		BeforeEach(func() {
			bindMountSourceCreator.CreateStub = func(containerDir string, _ bool) ([]garden.BindMount, error) {
				Expect(ioutil.WriteFile(filepath.Join(containerDir, "hosts"), nil, 0755)).To(Succeed())
				Expect(ioutil.WriteFile(filepath.Join(containerDir, "resolv.conf"), nil, 0755)).To(Succeed())
				return nil, nil
			}
			_, err := networkDepot.SetupBindMounts(logger, "my-container", true, "/path/to/rootfs")
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			Expect(networkDepot.Destroy(logger, "my-container")).To(Succeed())
		})

		It("deletes the bind mounts from the depot", func() {
			containerDir := filepath.Join(dir, "my-container")
			Expect(containerDir).To(BeADirectory())
			Expect(filepath.Join(containerDir, "hosts")).NotTo(BeARegularFile())
			Expect(filepath.Join(containerDir, "resolv.conf")).NotTo(BeARegularFile())
		})
	})
})
